package main

import (
	"sync"
	"time"

	"github.com/coder/websocket"
)

type Token struct {
	rel    int
	done   bool
	colour string
}

type TokenView struct {
	Rel  int  `json:"rel"`
	Done bool `json:"done"`
}

type Player struct {
	colour string
	client *Client
	name   string
}

type timeoutEvent struct {
	tok   string
	timer *time.Timer
}

type PlayerView struct {
	Colour string `json:"colour"`
	Name   string `json:"name"`
}

type Msg struct {
	Type      string                 `json:"type"`
	Room      string                 `json:"room,omitempty"`
	Token     int                    `json:"token,omitempty"`
	Colour    string                 `json:"colour,omitempty"`
	Turn      string                 `json:"turn,omitempty"`
	Dice      int                    `json:"dice,omitempty"`
	Board     map[string][]TokenView `json:"board,omitempty"`
	Session   string                 `json:"session,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Players   []PlayerView           `json:"players,omitempty"`
	Host      string                 `json:"host,omitempty"`
	Standings []string               `json:"standings,omitempty"`
	Moves     []int                  `json:"moves,omitempty"`
	Finished  []string               `json:"finished,omitempty"`
}

type Client struct {
	conn       *websocket.Conn
	send       chan Msg
	room       *Room
	colour     string
	sessiontok string
	name       string
}

type Room struct {
	code       string
	clients    map[*Client]bool
	players    map[string]*Player
	join       chan *Client
	leave      chan *Client
	actions    chan Action
	rejoin     chan *Client
	tokens     map[string][]*Token
	turn       string
	dice       int
	sixes      int
	done       chan struct{}
	started    bool
	host       string
	finished   []string
	timers     map[string]*time.Timer
	timeout    chan timeoutEvent
	startCount int
	over       bool
}

type Action struct {
	client  *Client
	Message Msg
}

type Hub struct {
	mu    sync.Mutex
	rooms map[string]*Room
}
