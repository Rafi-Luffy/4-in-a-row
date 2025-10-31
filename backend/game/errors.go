package game

import "errors"

var (
	ErrGameNotActive  = errors.New("game is not active")
	ErrNotYourTurn    = errors.New("not your turn")
	ErrInvalidColumn  = errors.New("invalid column")
	ErrColumnFull     = errors.New("column is full")
	ErrGameNotFound   = errors.New("game not found")
	ErrPlayerNotFound = errors.New("player not found")
	ErrGameFull       = errors.New("game is full")
)