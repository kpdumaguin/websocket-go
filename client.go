// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"chat/model"
	"chat/storage"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Request format
type Request struct {
	WalletAddress   string `json:"wallet_address"`
	GameId          string `json:"game_id"`
	TransactionHash string `json:"transaction_hash"`
	PlayerNumber    int    `json:"player_number"`
	Move            int    `json:"move"`
}

type RockPaperScissors struct {
	GameId         string `json:"game_id"`
	Player1Address string `json:"player1_address"`
	Player2Address string `json:"player2_address"`
	Player1Move    int    `json:"player1_move"`
	Player2Move    int    `json:"player2_move"`
	Winner         string `json:"winner"`
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte
}

// readPump pumps messages from the websocket connection to the hub.
//
// The application runs readPump in a per-connection goroutine. The application
// ensures that there is at most one reader on a connection by executing all
// reads from this goroutine.
func (c *Client) readPump(repo *storage.Repository) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })

	request := Request{}

	for {
		err := c.conn.ReadJSON(&request)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		err = SavePlayerMove(repo, request.GameId, request.PlayerNumber, request.Move)
		if err != nil {
			c.hub.unregister <- c
			c.conn.Close()
			break
		}

		// message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		message := []byte(request.WalletAddress + "[" + request.GameId + "] : " + "MOVE: " + strconv.Itoa(request.Move))
		c.hub.broadcast <- message

		rpsData, err := RoundChecker(repo, request.GameId)
		if err != nil {
			c.hub.unregister <- c
			c.conn.Close()
			break
		}

		// if rpsDAta is not empty - broadcast winner
		var msg string
		if rpsData.GameId != "" {
			// check mo kung sino panalo sa dalawa
			if rpsData.Player1Move == 3 {
				if rpsData.Player2Move == 1 {
					msg = "player 2 wins"
				} else {
					msg = "player 1 wins"
				}
			} else {
				if rpsData.Player1Move > rpsData.Player2Move {
					msg = "player 1 wins"
				} else {
					msg = "player 2 wins"
				}
			}
			c.hub.broadcast <- []byte(msg)
		}

	}
}

// writePump pumps messages from the hub to the websocket connection.
//
// A goroutine running writePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) writePump(repo *storage.Repository) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func serveWs(repo *storage.Repository, hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump(repo)
	go client.readPump(repo)
}

func SavePlayerMove(repo *storage.Repository, game_id string, player_no, move int) error {
	if player_no == 1 {
		err := repo.DB.Model(&model.RockPaperScissors{}).Where("game_id = ?", game_id).Update("player1_move", move).Error
		if err != nil {
			return err
		}
	} else {
		err := repo.DB.Model(&model.RockPaperScissors{}).Where("game_id = ?", game_id).Update("player2_move", move).Error
		if err != nil {
			return err
		}
	}
	return nil
}

// returns non-empty rps struct if both players cast their moves
func RoundChecker(repo *storage.Repository, game_id string) (RockPaperScissors, error) {
	model := RockPaperScissors{}
	res := repo.DB.Find(&model, "game_id = ? AND player1_move IS NOT NULL AND player2_move IS NOT NULL", game_id)
	if res.Error != nil {
		return model, res.Error
	}

	return model, nil
}
