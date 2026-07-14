package main

import "math/rand"

var colours = []string{"red", "green", "yellow", "blue"}
var startOffset = map[string]int{"red": 0, "green": 13, "yellow": 26, "blue": 39}
var diagonals = [][2]string{{"red", "yellow"}, {"green", "blue"}}

func newTokens(colour string) []*Token {
	tokens := make([]*Token, 4)
	for i := range tokens {
		tokens[i] = &Token{rel: -1, colour: colour}
	}
	return tokens
}

func (r *Room) assignColour() string {
	var free []string
	for _, col := range colours {
		if _, taken := r.tokens[col]; !taken {
			free = append(free, col)
		}
	}
	if len(free) == 0 {
		return ""
	}
	return free[rand.Intn(len(free))]
}

func (r *Room) nextColour() string {
	idx := 0
	for i, col := range colours {
		if col == r.turn {
			idx = i
			break
		}
	}
	for i := 1; i < len(colours); i++ {
		cand := colours[(idx+i)%len(colours)]
		if _, in := r.tokens[cand]; in && !r.isFinished(cand) {
			return cand
		}
	}
	return r.turn
}

func (r *Room) advanceTurn() {
	r.dice = 0
	r.sixes = 0
	r.turn = r.nextColour()
}

func (r *Room) firstColour() string {
	for _, col := range colours {
		if _, in := r.tokens[col]; in {
			return col
		}
	}
	return ""
}

func (r *Room) makeDiagonal() {
	var ps []*Player
	for _, p := range r.players {
		ps = append(ps, p)
	}
	rand.Shuffle(len(ps), func(i, j int) { ps[i], ps[j] = ps[j], ps[i] })

	pair := diagonals[rand.Intn(len(diagonals))]
	for i, p := range ps {
		p.colour = pair[i]
		p.client.colour = pair[i]
	}
	r.tokens = map[string][]*Token{
		pair[0]: newTokens(pair[0]),
		pair[1]: newTokens(pair[1]),
	}
}

func (r *Room) wouldStack(t *Token, dice int) bool {
	dest := t.rel + dice
	if t.rel == -1 {
		dest = 0
	}
	if dest > 50 || isSafe(dest) {
		return false
	}
	for _, o := range r.tokens[t.colour] {
		if o != t && o.rel == dest {
			return true
		}
	}
	return false
}

func (r *Room) movableTokens(colour string, dice int) []int {
	var out []int
	for i, t := range r.tokens[colour] {
		if canMove(t, dice) && !r.wouldStack(t, dice) {
			out = append(out, i)
		}
	}
	return out
}

func (r *Room) won(colour string) bool {
	for _, t := range r.tokens[colour] {
		if !t.done {
			return false
		}
	}
	return true
}

func throwDice() int {
	return rand.Intn(6) + 1
}

func canMove(t *Token, dice int) bool {
	if t.done || (t.rel+dice) > 56 {
		return false
	}
	if t.rel == -1 {
		return dice == 6
	}
	return true
}

func isSafe(rel int) bool {
	switch rel {
	case 0, 8, 13, 21, 26, 34, 39, 47:
		return true
	}
	return false
}

func (t *Token) getAbs() int {
	return (startOffset[t.colour] + t.rel) % 52
}

func (t *Token) handleMove(dice int, tokens map[string][]*Token) string {
	if !canMove(t, dice) {
		return "nomove"
	}
	if t.rel == -1 && dice == 6 {
		t.rel = 0
		return "moved"
	}
	t.rel += dice
	if t.rel == 56 {
		t.done = true
		return "home"
	}
	if t.rel < 51 {
		dest := t.getAbs()
		var here []*Token
		for colour, toks := range tokens {
			if colour == t.colour {
				continue
			}
			for _, enemy := range toks {
				if enemy.rel < 0 || enemy.rel > 50 {
					continue
				}
				if enemy.getAbs() == dest {
					here = append(here, enemy)
				}
			}
		}
		if len(here) == 1 && !isSafe(here[0].rel) {
			here[0].rel = -1
			return "capture"
		}
	}
	return "moved"
}

func (r *Room) isFinished(colour string) bool {
	for _, c := range r.finished {
		if c == colour {
			return true
		}
	}
	return false
}

func (r *Room) gameOver(trailing ...string) ([]string, bool) {
	remaining := 0
	var last string
	for col := range r.tokens {
		if !r.isFinished(col) {
			remaining++
			last = col
		}
	}
	if remaining > 1 {
		return nil, false
	}
	standings := append([]string{}, r.finished...)
	if remaining == 1 {
		standings = append(standings, last)
	}
	return append(standings, trailing...), true
}
