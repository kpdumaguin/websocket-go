package model

import "gorm.io/gorm"

type RockPaperScissors struct {
	ID             uint    `gorm:"primary key;autoIncrement" json:"id"`
	GameId         *string `json:"game_id"`
	Player1Address *string `json:"player1_address"`
	Player2Address *string `json:"player2_address"`
	Player1Move    *int    `json:"player1_move"`
	Player2Move    *int    `json:"player2_move"`
	Winner         *string `json:"winner"`
}

func MigrateTables(db *gorm.DB) error {
	err := db.AutoMigrate(&RockPaperScissors{})
	return err
}
