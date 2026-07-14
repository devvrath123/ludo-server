package main

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"net/http"
	"slices"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

var notFoundErr = errors.New("Room does not exist! Create a new one")

const codeLetters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

const graceTimeout = 30 * time.Second

func newHub() *Hub {
	return &Hub{rooms: make(map[string]*Room)}
}

func randomCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = codeLetters[rand.Intn(len(codeLetters))]
	}
	return string(b)
}

func newRoom(code string) *Room {
	return &Room{
		code:    code,
		clients: make(map[*Client]bool),
		players: make(map[string]*Player),
		join:    make(chan *Client),
		leave:   make(chan *Client),
		actions: make(chan Action),
		rejoin:  make(chan *Client),
		tokens:  make(map[string][]*Token),
		timers:  make(map[string]*time.Timer),
		timeout: make(chan timeoutEvent),
		done:    make(chan struct{}),
	}
}

func (h *Hub) createRoom() *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	var code string
	for {
		code = randomCode(6)
		if _, exists := h.rooms[code]; !exists {
			break
		}
	}
	r := newRoom(code)
	h.rooms[code] = r
	go r.run()
	return r
}

func (h *Hub) getRoom(code string) (*Room, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[code]
	if !ok {
		return nil, notFoundErr
	}
	return r, nil
}

func (h *Hub) removeRoom(code string) {
	h.mu.Lock()
	delete(h.rooms, code)
	h.mu.Unlock()
}

func (r *Room) run() {
	const emptyGrace = 5 * time.Minute

	var idle *time.Timer
	var idleC <-chan time.Time

	arm := func() {
		if idle == nil {
			idle = time.NewTimer(emptyGrace)
		} else {
			idle.Reset(emptyGrace)
		}
		idleC = idle.C
	}
	disarm := func() {
		if idle != nil {
			idle.Stop()
		}
		idleC = nil
	}

	arm()

	for {
		select {
		case c := <-r.join:
			if r.started {
				c.send <- Msg{Type: "error", Room: r.code, Reason: "started"}
				continue
			}
			colour := r.assignColour()
			if colour == "" {
				c.send <- Msg{Type: "error", Room: r.code, Reason: "full"}
				continue
			}
			disarm()
			tok := randomCode(16)
			c.colour = colour
			c.sessiontok = tok
			if r.host == "" {
				r.host = tok
			}
			r.clients[c] = true
			r.tokens[colour] = newTokens(colour)
			r.players[tok] = &Player{colour: colour, client: c, name: c.name}
			c.send <- Msg{Type: "joined", Room: r.code, Colour: colour, Session: tok}
			r.broadcast(Msg{Type: "lobby", Players: r.roster(), Host: r.hostColour()})

		case c := <-r.rejoin:
			p, ok := r.players[c.sessiontok]
			if !ok {
				c.send <- Msg{Type: "error", Room: r.code}
				continue
			}
			disarm()
			if p.client != nil && p.client != c {
				delete(r.clients, p.client)
				p.client.conn.CloseNow()
			}
			p.client = c
			r.stopGraceTimer(c.sessiontok)
			c.colour = p.colour
			r.clients[c] = true
			var moves []int
			if r.dice > 0 {
				moves = r.movableTokens(r.turn, r.dice)
			}
			c.send <- Msg{Type: "resync", Room: r.code, Colour: p.colour, Board: r.snapshot(), Turn: r.turn, Dice: r.dice, Moves: moves, Players: r.roster(), Finished: r.finished}
			r.broadcast(Msg{Type: "playerRejoined", Colour: p.colour})

		case ev := <-r.timeout:
			r.handleTimeout(ev)

		case c := <-r.leave:
			delete(r.clients, c)
			close(c.send)
			if p, ok := r.players[c.sessiontok]; ok && p.client == c {
				if r.started {
					p.client = nil
					if !r.over {
						r.broadcast(Msg{Type: "playerLeft", Colour: c.colour})
						r.startGraceTimer(c.sessiontok)
					}
				} else {
					delete(r.players, c.sessiontok)
					delete(r.tokens, c.colour)
					if r.host == c.sessiontok {
						r.host = ""
						for tok := range r.players {
							r.host = tok
							break
						}
					}
					r.broadcast(Msg{Type: "lobby", Players: r.roster(), Host: r.hostColour()})
				}
			}
			if len(r.clients) == 0 {
				if !r.started {
					hub.removeRoom(r.code)
					close(r.done)
					return
				}
				arm()
			}

		case a := <-r.actions:
			r.handle(a)

		case <-idleC:
			hub.removeRoom(r.code)
			close(r.done)
			return
		}
	}
}

func (r *Room) broadcast(m Msg) {
	for c := range r.clients {
		select {
		case c.send <- m:
		default:
		}
	}
}

func (r *Room) handle(a Action) {
	switch a.Message.Type {
	case "start":
		if a.client.sessiontok != r.host {
			return
		}
		if r.started || len(r.tokens) < 2 {
			return
		}
		r.started = true
		if len(r.tokens) == 2 {
			r.makeDiagonal()
		}
		r.startCount = len(r.tokens)
		r.turn = r.firstColour()
		board := r.snapshot()
		roster := r.roster()
		for c := range r.clients {
			select {
			case c.send <- Msg{Type: "start", Turn: r.turn, Board: board, Colour: c.colour, Players: roster}:
			default:
			}
		}
	case "roll":
		if a.client.colour != r.turn || r.dice != 0 {
			return
		}
		r.dice = throwDice()
		if r.dice == 6 {
			r.sixes++
		} else {
			r.sixes = 0
		}
		if r.sixes == 3 {
			r.broadcast(Msg{Type: "rolled", Turn: r.turn, Dice: r.dice})
			r.advanceTurn()
			r.broadcast(Msg{Type: "state", Board: r.snapshot(), Turn: r.turn, Finished: r.finished})
			return
		}
		moves := r.movableTokens(r.turn, r.dice)
		r.broadcast(Msg{Type: "rolled", Turn: r.turn, Dice: r.dice, Moves: moves})
		if len(moves) == 0 {
			r.advanceTurn()
			r.broadcast(Msg{Type: "state", Board: r.snapshot(), Turn: r.turn, Finished: r.finished})
		}
	case "move":
		if a.client.colour != r.turn || r.dice == 0 {
			return
		}
		toks := r.tokens[a.client.colour]
		i := a.Message.Token
		if !slices.Contains(r.movableTokens(a.client.colour, r.dice), i) {
			return
		}
		result := toks[i].handleMove(r.dice, r.tokens)
		if result == "nomove" {
			return
		}

		goAgain := r.dice == 6 || result == "capture" || result == "home"
		r.dice = 0
		if result == "home" && r.won(a.client.colour) {
			r.finished = append(r.finished, a.client.colour)
			goAgain = false
			if s, ok := r.gameOver(); ok {
				r.over = true
				r.broadcast(Msg{Type: "over", Standings: s, Board: r.snapshot()})
				return
			}
		}
		if !goAgain {
			r.advanceTurn()
		}
		r.broadcast(Msg{Type: "state", Board: r.snapshot(), Turn: r.turn, Finished: r.finished})
	case "quit":
		if _, ok := r.players[a.client.sessiontok]; !ok || !r.started {
			return
		}
		r.forfeit(a.client.sessiontok)
	}
}

func (c *Client) readPump() {
	ctx := context.Background()
	defer func() {
		if c.room != nil {
			select {
			case c.room.leave <- c:
			case <-c.room.done:
				close(c.send)
			}
		} else {
			close(c.send)
		}
		c.conn.CloseNow()
	}()
	for {
		var m Msg
		if err := wsjson.Read(ctx, c.conn, &m); err != nil {
			return
		}
		switch m.Type {
		case "create":
			c.name = m.Name
			c.room = hub.createRoom()
			c.room.join <- c
			c.send <- Msg{Type: "created", Room: c.room.code}
		case "join":
			r, err := hub.getRoom(m.Room)
			if err != nil {
				c.send <- Msg{Type: "error", Room: m.Room, Reason: "notfound"}
				continue
			}
			c.room = r
			c.name = m.Name
			select {
			case r.join <- c:
			case <-r.done:
				c.send <- Msg{Type: "error", Room: m.Room}
				c.room = nil
			}
		case "rejoin":
			r, err := hub.getRoom(m.Room)
			if err != nil {
				c.send <- Msg{Type: "error", Room: m.Room, Reason: "notfound"}
				continue
			}
			c.room = r
			c.sessiontok = m.Session
			select {
			case r.rejoin <- c:
			case <-r.done:
				c.send <- Msg{Type: "error", Room: m.Room}
				c.room = nil
			}
		case "ping":
			select {
			case c.send <- Msg{Type: "pong"}:
			default:
			}
		default:
			if c.room != nil {
				c.room.actions <- Action{client: c, Message: m}
			}
		}
	}
}

func (c *Client) writePump() {
	ctx := context.Background()
	for m := range c.send {
		if err := wsjson.Write(ctx, c.conn, m); err != nil {
			return
		}
	}
}

func (r *Room) roster() []PlayerView {
	out := make([]PlayerView, 0, len(r.players))
	for _, p := range r.players {
		out = append(out, PlayerView{Colour: p.colour, Name: p.name})
	}
	return out
}

func (r *Room) hostColour() string {
	if p, ok := r.players[r.host]; ok {
		return p.colour
	}
	return ""
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{
			"localhost", "localhost:*",
			"wails.localhost", "wails.localhost:*",
			"wails",
		},
	})
	if err != nil {
		log.Println("accept:", err)
		return
	}
	c := &Client{conn: conn, send: make(chan Msg, 16)}
	go c.writePump()
	c.readPump()
}

func (r *Room) snapshot() map[string][]TokenView {
	board := make(map[string][]TokenView, len(r.tokens))
	for colour, toks := range r.tokens {
		views := make([]TokenView, len(toks))
		for i, t := range toks {
			views[i] = TokenView{Rel: t.rel, Done: t.done}
		}
		board[colour] = views
	}
	return board
}

func (r *Room) startGraceTimer(tok string) {
	r.stopGraceTimer(tok)
	var t *time.Timer
	t = time.AfterFunc(graceTimeout, func() {
		select {
		case r.timeout <- timeoutEvent{tok, t}:
		case <-r.done:
		}
	})
	r.timers[tok] = t
}

func (r *Room) stopGraceTimer(tok string) {
	if t, ok := r.timers[tok]; ok {
		t.Stop()
		delete(r.timers, tok)
	}
}

func (r *Room) handleTimeout(ev timeoutEvent) {
	if r.timers[ev.tok] != ev.timer {
		return
	}
	delete(r.timers, ev.tok)
	if r.over {
		return
	}
	if p, ok := r.players[ev.tok]; !ok || p.client != nil {
		return
	}
	r.forfeit(ev.tok)
}

func (r *Room) forfeit(tok string) {
	p, ok := r.players[tok]
	if !ok {
		return
	}
	col := p.colour
	delete(r.players, tok)
	delete(r.tokens, col)
	r.stopGraceTimer(tok)
	r.broadcast(Msg{Type: "playerLeft", Colour: col})

	var trailing []string
	if r.startCount == 2 {
		trailing = []string{col}
	}
	if s, ok := r.gameOver(trailing...); ok {
		r.over = true
		r.broadcast(Msg{Type: "over", Standings: s, Board: r.snapshot()})
		return
	}
	if r.turn == col {
		r.advanceTurn()
	}
	var moves []int
	if r.dice > 0 {
		moves = r.movableTokens(r.turn, r.dice)
	}
	r.broadcast(Msg{Type: "state", Board: r.snapshot(), Turn: r.turn, Dice: r.dice, Moves: moves, Finished: r.finished})
}
