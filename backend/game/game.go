package game

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const (
	ROWS    = 6
	COLS    = 7
	EMPTY   = 0
	PLAYER1 = 1
	PLAYER2 = 2
)

type Game struct {
	ID          string     `json:"id"`
	Board       [][]int    `json:"board"`
	CurrentTurn int        `json:"currentTurn"`
	Status      string     `json:"status"` // "waiting", "playing", "finished"
	Winner      int        `json:"winner"`
	Player1     *Player    `json:"player1"`
	Player2     *Player    `json:"player2"`
	CreatedAt   time.Time  `json:"createdAt"`
	LastMove    time.Time  `json:"lastMove"`
	IsBot       bool       `json:"isBot"`
}

type Player struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	IsBot    bool   `json:"isBot"`
}

type Move struct {
	GameID string `json:"gameId"`
	Player int    `json:"player"`
	Column int    `json:"column"`
	Row    int    `json:"row"`
}

type GameEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewGame(player1 *Player) *Game {
	board := make([][]int, ROWS)
	for i := range board {
		board[i] = make([]int, COLS)
	}

	return &Game{
		ID:          uuid.New().String(),
		Board:       board,
		CurrentTurn: PLAYER1,
		Status:      "waiting",
		Winner:      0,
		Player1:     player1,
		CreatedAt:   time.Now(),
		LastMove:    time.Now(),
	}
}

func (g *Game) AddPlayer2(player *Player) {
	g.Player2 = player
	g.Status = "playing"
	g.IsBot = player.IsBot
}

func (g *Game) MakeMove(column int, player int) (*Move, error) {
	if g.Status != "playing" {
		return nil, ErrGameNotActive
	}

	if g.CurrentTurn != player {
		return nil, ErrNotYourTurn
	}

	if column < 0 || column >= COLS {
		return nil, ErrInvalidColumn
	}

	// Find the lowest empty row in the column
	row := -1
	for r := ROWS - 1; r >= 0; r-- {
		if g.Board[r][column] == EMPTY {
			row = r
			break
		}
	}

	if row == -1 {
		return nil, ErrColumnFull
	}

	// Place the piece
	g.Board[row][column] = player
	g.LastMove = time.Now()

	move := &Move{
		GameID: g.ID,
		Player: player,
		Column: column,
		Row:    row,
	}

	// Check for win
	if g.checkWin(row, column, player) {
		g.Status = "finished"
		g.Winner = player
	} else if g.isBoardFull() {
		g.Status = "finished"
		g.Winner = 0 // Draw
	} else {
		// Switch turns
		if g.CurrentTurn == PLAYER1 {
			g.CurrentTurn = PLAYER2
		} else {
			g.CurrentTurn = PLAYER1
		}
	}

	return move, nil
}

func (g *Game) checkWin(row, col, player int) bool {
	directions := [][]int{
		{0, 1},  // horizontal
		{1, 0},  // vertical
		{1, 1},  // diagonal /
		{1, -1}, // diagonal \
	}

	for _, dir := range directions {
		count := 1 // Count the current piece
		
		// Check in positive direction
		for i := 1; i < 4; i++ {
			newRow := row + dir[0]*i
			newCol := col + dir[1]*i
			if newRow < 0 || newRow >= ROWS || newCol < 0 || newCol >= COLS {
				break
			}
			if g.Board[newRow][newCol] == player {
				count++
			} else {
				break
			}
		}
		
		// Check in negative direction
		for i := 1; i < 4; i++ {
			newRow := row - dir[0]*i
			newCol := col - dir[1]*i
			if newRow < 0 || newRow >= ROWS || newCol < 0 || newCol >= COLS {
				break
			}
			if g.Board[newRow][newCol] == player {
				count++
			} else {
				break
			}
		}
		
		if count >= 4 {
			return true
		}
	}
	
	return false
}

func (g *Game) isBoardFull() bool {
	for col := 0; col < COLS; col++ {
		if g.Board[0][col] == EMPTY {
			return false
		}
	}
	return true
}

func (g *Game) GetValidMoves() []int {
	var moves []int
	for col := 0; col < COLS; col++ {
		if g.Board[0][col] == EMPTY {
			moves = append(moves, col)
		}
	}
	return moves
}

func (g *Game) ToJSON() []byte {
	data, _ := json.Marshal(g)
	return data
}