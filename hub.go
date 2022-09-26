// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"chat/model"
	"chat/storage"
	"fmt"
)

// Hub maintains the set of active clients and broadcasts messages to the
// clients.
type Hub struct {
	// Registered clients.
	clients map[*Client]bool

	// Inbound messages from the clients.
	broadcast chan []byte

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run(repo *storage.Repository) {
	for {
		select {
		case client := <-h.register:
			wallet_address := "oxnice1"
			isplayer, err := isPlayer(repo, wallet_address)
			if err != nil {
				delete(h.clients, client)
				close(client.send)
				fmt.Printf("Cannot check %v user if player.\n", wallet_address)
			}
			if isplayer {
				h.clients[client] = true
			} else {
				delete(h.clients, client)
				close(client.send)
				fmt.Printf("Player %v is not a player \n", wallet_address)
			}
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}

func isPlayer(r *storage.Repository, wallet_address string) (bool, error) {
	model := model.RockPaperScissors{}
	res := r.DB.Find(&model, "player1_address = ? OR player2_address = ?", wallet_address, wallet_address)
	if res.Error != nil {
		return false, res.Error
	}

	if res.RowsAffected > 0 {
		return true, nil
	}

	return false, nil
}
