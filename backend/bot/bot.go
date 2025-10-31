package bot

import (
	"math/rand"
	"time"
)

const (
	ROWS    = 6
	COLS    = 7
	EMPTY   = 0
	PLAYER1 = 1
	PLAYER2 = 2
)

type Bot struct {
	rand *rand.Rand
}

func NewBot() *Bot {
	return &Bot{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (b *Bot) GetBestMove(board [][]int, player int) int {
	return b.GetBestMoveWithDifficulty(board, player, 0)
}

func (b *Bot) GetBestMoveWithDifficulty(board [][]int, player int, playerWins int) int {
	// Calculate difficulty level based on player's consecutive wins
	difficultyLevel := playerWins
	if difficultyLevel > 5 {
		difficultyLevel = 5 // Cap at level 5
	}
	
	// At higher difficulty levels, bot makes fewer mistakes
	mistakeChance := 0.3 - (float64(difficultyLevel) * 0.05) // 30% down to 5% mistake chance
	if mistakeChance < 0.05 {
		mistakeChance = 0.05
	}
	
	// Occasionally make a suboptimal move at lower difficulties
	if difficultyLevel < 3 && b.rand.Float64() < mistakeChance {
		return b.makeSuboptimalMove(board, player)
	}

	// 1. Check if bot can win immediately (always prioritize)
	for col := 0; col < COLS; col++ {
		if b.canDropPiece(board, col) {
			testBoard := b.copyBoard(board)
			row := b.dropPiece(testBoard, col, player)
			if b.checkWin(testBoard, row, col, player) {
				return col
			}
		}
	}

	// 2. Block opponent from winning (higher difficulty = better blocking)
	opponent := PLAYER1
	if player == PLAYER1 {
		opponent = PLAYER2
	}

	blockingMoves := []int{}
	for col := 0; col < COLS; col++ {
		if b.canDropPiece(board, col) {
			testBoard := b.copyBoard(board)
			row := b.dropPiece(testBoard, col, opponent)
			if b.checkWin(testBoard, row, col, opponent) {
				blockingMoves = append(blockingMoves, col)
			}
		}
	}
	
	// At higher difficulty, always block. At lower difficulty, sometimes miss blocks
	if len(blockingMoves) > 0 {
		if difficultyLevel >= 2 || b.rand.Float64() > mistakeChance {
			return blockingMoves[0]
		}
	}

	// 3. Try to create winning opportunities (better at higher difficulty)
	bestCol := b.findBestStrategicMove(board, player, difficultyLevel)
	if bestCol != -1 {
		return bestCol
	}

	// 4. Prefer center columns (more strategic at higher difficulty)
	centerCols := []int{3, 2, 4, 1, 5, 0, 6}
	if difficultyLevel >= 1 {
		for _, col := range centerCols {
			if b.canDropPiece(board, col) {
				return col
			}
		}
	}

	// 5. Fallback to random valid move
	validMoves := b.getValidMoves(board)
	if len(validMoves) > 0 {
		return validMoves[b.rand.Intn(len(validMoves))]
	}

	return 0
}

func (b *Bot) makeSuboptimalMove(board [][]int, player int) int {
	validMoves := b.getValidMoves(board)
	if len(validMoves) == 0 {
		return 0
	}
	
	// Prefer edge columns for suboptimal play
	edgeCols := []int{0, 6, 1, 5}
	for _, col := range edgeCols {
		if b.canDropPiece(board, col) {
			return col
		}
	}
	
	return validMoves[b.rand.Intn(len(validMoves))]
}

func (b *Bot) findBestStrategicMove(board [][]int, player int, difficultyLevel int) int {
	bestScore := -1
	bestCol := -1

	for col := 0; col < COLS; col++ {
		if b.canDropPiece(board, col) {
			testBoard := b.copyBoard(board)
			row := b.dropPiece(testBoard, col, player)
			score := b.evaluatePosition(testBoard, row, col, player)
			
			// At higher difficulty, look ahead more moves
			if difficultyLevel >= 3 {
				score += b.evaluateFuturePositions(testBoard, player, 2)
			} else if difficultyLevel >= 1 {
				score += b.evaluateFuturePositions(testBoard, player, 1)
			}
			
			if score > bestScore {
				bestScore = score
				bestCol = col
			}
		}
	}

	return bestCol
}

func (b *Bot) evaluateFuturePositions(board [][]int, player int, depth int) int {
	if depth <= 0 {
		return 0
	}
	
	totalScore := 0
	validMoves := b.getValidMoves(board)
	
	for _, col := range validMoves {
		testBoard := b.copyBoard(board)
		row := b.dropPiece(testBoard, col, player)
		score := b.evaluatePosition(testBoard, row, col, player)
		score += b.evaluateFuturePositions(testBoard, player, depth-1)
		totalScore += score
	}
	
	if len(validMoves) > 0 {
		return totalScore / len(validMoves)
	}
	
	return 0
}

func (b *Bot) evaluatePosition(board [][]int, row, col, player int) int {
	score := 0
	
	// Check all directions for potential connections
	directions := [][]int{
		{0, 1},  // horizontal
		{1, 0},  // vertical
		{1, 1},  // diagonal /
		{1, -1}, // diagonal \
	}

	for _, dir := range directions {
		score += b.evaluateDirection(board, row, col, dir[0], dir[1], player)
	}

	// Bonus for center column
	if col == 3 {
		score += 3
	} else if col == 2 || col == 4 {
		score += 2
	}

	return score
}

func (b *Bot) evaluateDirection(board [][]int, row, col, deltaRow, deltaCol, player int) int {
	count := 1
	openEnds := 0

	// Check positive direction
	for i := 1; i < 4; i++ {
		newRow := row + deltaRow*i
		newCol := col + deltaCol*i
		if newRow < 0 || newRow >= ROWS || newCol < 0 || newCol >= COLS {
			break
		}
		if board[newRow][newCol] == player {
			count++
		} else if board[newRow][newCol] == EMPTY {
			openEnds++
			break
		} else {
			break
		}
	}

	// Check negative direction
	for i := 1; i < 4; i++ {
		newRow := row - deltaRow*i
		newCol := col - deltaCol*i
		if newRow < 0 || newRow >= ROWS || newCol < 0 || newCol >= COLS {
			break
		}
		if board[newRow][newCol] == player {
			count++
		} else if board[newRow][newCol] == EMPTY {
			openEnds++
			break
		} else {
			break
		}
	}

	// Score based on count and open ends
	if count >= 4 {
		return 1000 // Winning move
	} else if count == 3 && openEnds > 0 {
		return 50
	} else if count == 2 && openEnds > 0 {
		return 10
	} else if count == 1 && openEnds > 1 {
		return 1
	}

	return 0
}

func (b *Bot) canDropPiece(board [][]int, col int) bool {
	return col >= 0 && col < COLS && board[0][col] == EMPTY
}

func (b *Bot) dropPiece(board [][]int, col, player int) int {
	for row := ROWS - 1; row >= 0; row-- {
		if board[row][col] == EMPTY {
			board[row][col] = player
			return row
		}
	}
	return -1
}

func (b *Bot) copyBoard(board [][]int) [][]int {
	newBoard := make([][]int, ROWS)
	for i := range newBoard {
		newBoard[i] = make([]int, COLS)
		copy(newBoard[i], board[i])
	}
	return newBoard
}

func (b *Bot) checkWin(board [][]int, row, col, player int) bool {
	directions := [][]int{
		{0, 1},  // horizontal
		{1, 0},  // vertical
		{1, 1},  // diagonal /
		{1, -1}, // diagonal \
	}

	for _, dir := range directions {
		count := 1

		// Check positive direction
		for i := 1; i < 4; i++ {
			newRow := row + dir[0]*i
			newCol := col + dir[1]*i
			if newRow < 0 || newRow >= ROWS || newCol < 0 || newCol >= COLS {
				break
			}
			if board[newRow][newCol] == player {
				count++
			} else {
				break
			}
		}

		// Check negative direction
		for i := 1; i < 4; i++ {
			newRow := row - dir[0]*i
			newCol := col - dir[1]*i
			if newRow < 0 || newRow >= ROWS || newCol < 0 || newCol >= COLS {
				break
			}
			if board[newRow][newCol] == player {
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

func (b *Bot) getValidMoves(board [][]int) []int {
	var moves []int
	for col := 0; col < COLS; col++ {
		if b.canDropPiece(board, col) {
			moves = append(moves, col)
		}
	}
	return moves
}