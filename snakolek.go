package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"time"

	termbox "github.com/nsf/termbox-go"
)

var host = "http://snakolek.ironsys.pl"
var highScoresURL = host + "/api/high-scores"
var messagesURL = host + "/api/messages"
var appName, appVersion, appPlatform = "Snakolek", "0.991", runtime.GOOS
var gEnv gameEnvironment
var gSt gameState
var randomGenerator *rand.Rand
var redraw *time.Ticker
var ironsys, flc string

// struct for storing pair of coordinates
type coords struct {
	x int
	y int
}

type spinner struct {
	state   bool
	counter int
	xPos    int
	yPos    int
	fgColor termbox.Attribute
	bgColor termbox.Attribute
}

// struct for storing game environment
type gameEnvironment struct {
	consoleWidth           int
	consoleHeight          int
	boardWidth             int
	boardHeight            int
	soundOn                bool
	tickerFactor           float32
	playerName             string
	spinner                spinner
	isHighScoreRequest     bool
	highScoreRequestResult bool
}

// struct for storing game state
type gameState struct {
	snakeDirX          int
	snakeDirY          int
	inputDirX          int
	inputDirY          int
	snakeHeadCoords    coords
	snakeCoords        []coords
	fruitCoords        coords
	fruitDistance      int
	stepsToFruit       int
	score              int
	isFruit            bool
	isSpecialFruit     bool
	eliMode            bool
	gameStarted        bool
	gamePaused         bool
	gameOver           bool
	serverContentShown bool
	tickerDelay        float32
	fruits             int
	specialFruits      int
	startTimestamp     int64
	endTimestamp       int64
}

type highScore struct {
	PlayerName     string `json:"player_name"`
	Score          int    `json:"score"`
	EliMode        bool   `json:"eli_mode"`
	BoardWidth     int    `json:"board_width"`
	BoardHeight    int    `json:"board_height"`
	Fruits         int    `json:"fruits"`
	SpecialFruits  int    `json:"special_fruits"`
	TickerDelay    int    `json:"ticker_delay"`
	StartTimestamp int64  `json:"start_timestamp"`
	EndTimestamp   int64  `json:"end_timestamp"`
	AppVersion     string `json:"app_version"`
	Goos           string `json:"goos"`
}

type highScoreWrapper struct {
	Signature string    `json:"signature"`
	Data      highScore `json:"data"`
}

type onlineHighScore struct {
	PlayerName    string `json:"player_name"`
	Score         int    `json:"score"`
	EliMode       bool   `json:"eli_mode"`
	BoardWidth    int    `json:"board_width"`
	BoardHeight   int    `json:"board_height"`
	Fruits        int    `json:"fruits"`
	SpecialFruits int    `json:"special_fruits"`
	Duration      int    `json:"duration"`
	CreatedAt     string `json:"created_at"`
}

type onlineMessage struct {
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

var onlineHighScores []onlineHighScore

func initGameEnvironment() {
	consoleWidth, consoleHeight := termbox.Size()

	gEnv = gameEnvironment{
		consoleWidth:  consoleWidth,
		consoleHeight: consoleHeight,
		boardWidth:    consoleWidth / 2,
		boardHeight:   consoleHeight - 1,
		soundOn:       false,
		tickerFactor:  0.995,
		playerName:    "",
		spinner: spinner{
			state:   false,
			counter: 0,
			xPos:    0,
			yPos:    0,
			fgColor: termbox.ColorDefault,
			bgColor: termbox.ColorDefault,
		},
		isHighScoreRequest:     false,
		highScoreRequestResult: false,
	}
}

func initFreshGameState(eliMode bool) {
	snakeDirX, snakeDirY := generateRandomSnakeDirection()
	snakeHeadCoords := coords{
		x: gEnv.boardWidth / 2,
		y: gEnv.boardHeight / 2,
	}

	snakeCoords := make([]coords, 0, 0)
	snakeCoords = append(snakeCoords, snakeHeadCoords)

	gSt = gameState{
		snakeDirX:          snakeDirX,
		snakeDirY:          snakeDirY,
		inputDirX:          snakeDirX,
		inputDirY:          snakeDirY,
		snakeHeadCoords:    snakeHeadCoords,
		snakeCoords:        snakeCoords,
		fruitCoords:        coords{},
		fruitDistance:      0,
		stepsToFruit:       0,
		score:              0,
		isFruit:            false,
		isSpecialFruit:     false,
		eliMode:            eliMode,
		gameStarted:        false,
		gamePaused:         false,
		gameOver:           false,
		serverContentShown: false,
		tickerDelay:        100,
		fruits:             0,
		specialFruits:      0,
		startTimestamp:     time.Now().UTC().Unix(),
		endTimestamp:       0,
	}
}

// @TODO highscores (send encrypted with public key)
func main() {
	err := termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()

	termbox.HideCursor()

	randSource := rand.NewSource(time.Now().UnixNano())
	randomGenerator = rand.New(randSource)

	initGameEnvironment()
	initFreshGameState(false)
	if !printServerMessages() {
		printCleanBoard()
		printGameIntro()
	}

	eventQueue := make(chan termbox.Event)
	go func() {
		for {
			eventQueue <- termbox.PollEvent()
		}
	}()

	redraw = time.NewTicker(time.Duration(gSt.tickerDelay) * time.Millisecond)

loop:
	for {
		select {
		case ev := <-eventQueue:
			if ev.Type == termbox.EventKey {
				switch ev.Key {
				case termbox.KeyArrowLeft:
					interruptPause()
					if gSt.snakeDirX == 0 {
						gSt.inputDirX, gSt.inputDirY = -1, 0
					}
				case termbox.KeyArrowRight:
					interruptPause()
					if gSt.snakeDirX == 0 {
						gSt.inputDirX, gSt.inputDirY = 1, 0
					}
				case termbox.KeyArrowUp:
					interruptPause()
					if gSt.snakeDirY == 0 {
						gSt.inputDirX, gSt.inputDirY = 0, -1
					}
				case termbox.KeyArrowDown:
					interruptPause()
					if gSt.snakeDirY == 0 {
						gSt.inputDirX, gSt.inputDirY = 0, 1
					}
				case termbox.KeyCtrlP:
					if !gSt.gamePaused && !gSt.gameOver && gSt.gameStarted {
						gSt.gamePaused = true
						printGamePausedInfo()
					}
				case termbox.KeyCtrlS:
					gEnv.soundOn = !gEnv.soundOn
					printStatusBar()
					beep()
				case termbox.KeySpace:
					if gSt.gameOver || gSt.serverContentShown {
						printCleanBoard()
						printGameIntro()
						gSt.serverContentShown = false
					}
				case termbox.KeyCtrlR:
					if !gSt.gameStarted && !gSt.gameOver && !gSt.serverContentShown {
						startNewGame(false)
					}
				case termbox.KeyCtrlE:
					if !gSt.gameStarted && !gSt.gameOver && !gSt.serverContentShown {
						startNewGame(true)
					}
				case termbox.KeyCtrlH:
					if !gSt.gameStarted && !gSt.gameOver {
						showHighScores()
					}
				case termbox.KeyCtrlQ:
					// quit the game
					break loop
				}

			}
		case <-redraw.C:
			if gSt.gameStarted && !gSt.gamePaused && !gSt.gameOver {
				eraseSnake(gSt.snakeCoords)
				gSt.snakeDirX, gSt.snakeDirY = gSt.inputDirX, gSt.inputDirY

				gSt.snakeHeadCoords.x += gSt.snakeDirX
				gSt.snakeHeadCoords.y += gSt.snakeDirY

				// allow going "through" the walls in Eli mode
				if gSt.eliMode {
					if gSt.snakeHeadCoords.x < 0 {
						gSt.snakeHeadCoords.x = gEnv.boardWidth - 1
					} else if gSt.snakeHeadCoords.x >= gEnv.boardWidth {
						gSt.snakeHeadCoords.x = 0
					}

					if gSt.snakeHeadCoords.y < 0 {
						gSt.snakeHeadCoords.y = gEnv.boardHeight - 1
					} else if gSt.snakeHeadCoords.y >= gEnv.boardHeight {
						gSt.snakeHeadCoords.y = 0
					}
				}

				// move snakes head
				gSt.snakeCoords = append(gSt.snakeCoords, gSt.snakeHeadCoords)
				gSt.stepsToFruit++

				if gSt.isFruit && gSt.isSpecialFruit && gSt.stepsToFruit > gSt.fruitDistance+10 {
					eraseFruit(gSt.fruitCoords)
					gSt.isFruit = false
					gSt.isSpecialFruit = false
				}

				// check if snakes head went over the "fruit"
				if gSt.isFruit && gSt.snakeHeadCoords.x == gSt.fruitCoords.x && gSt.snakeHeadCoords.y == gSt.fruitCoords.y {
					scoreDelta := 10 * gSt.fruitDistance / gSt.stepsToFruit
					if gSt.isSpecialFruit {
						scoreDelta *= 10
						gSt.specialFruits++
					} else {
						gSt.fruits++
					}
					gSt.score += scoreDelta
					gSt.tickerDelay *= gEnv.tickerFactor
					redraw.Stop()
					ratio := 1
					if gSt.isSpecialFruit {
						ratio = 2
					}
					redraw = time.NewTicker(time.Duration(gSt.tickerDelay/float32(ratio)) * time.Millisecond)
					gSt.isFruit = false
					gSt.isSpecialFruit = false
					printStatusBar()
					beep()
				} else {
					if len(gSt.snakeCoords) > 1 {
						// remove snakes tail if no fruit eaten
						gSt.snakeCoords = append(gSt.snakeCoords[:0], gSt.snakeCoords[1:]...)
					}
				}

				// check if snakes head went ouside of the area (not in Eli mode)
				if !gSt.eliMode {
					if gSt.snakeHeadCoords.x < 0 || gSt.snakeHeadCoords.x >= gEnv.boardWidth || gSt.snakeHeadCoords.y < 0 || gSt.snakeHeadCoords.y >= gEnv.boardHeight {
						handleGameOver()
					}
				}

				// check if snakes head went over the snakes body
				if len(gSt.snakeCoords) > 1 && areCoordsInSlice(gSt.snakeHeadCoords, gSt.snakeCoords[0:len(gSt.snakeCoords)-1]) {
					handleGameOver()
				}

				if !gSt.gameOver && gSt.gameStarted {
					printSnake(gSt.snakeCoords)

					// if no fruit, generate and print new one
					if !gSt.isFruit {
						for {
							gSt.fruitCoords.x = randomGenerator.Intn(gEnv.boardWidth)
							gSt.fruitCoords.y = randomGenerator.Intn(gEnv.boardHeight)

							if !areCoordsInSlice(gSt.fruitCoords, gSt.snakeCoords) {
								break
							}
						}

						gSt.isFruit = true
						gSt.isSpecialFruit = randomGenerator.Intn(100) > 84
						gSt.fruitDistance = int(math.Abs(float64(gSt.snakeHeadCoords.x-gSt.fruitCoords.x)) + math.Abs(float64(gSt.snakeHeadCoords.y-gSt.fruitCoords.y)))
						gSt.stepsToFruit = 0
						printFruit(gSt.fruitCoords)
					}
				}
			}
			termbox.Flush()
		}
	}
}

func startNewGame(eliMode bool) {
	initFreshGameState(eliMode)
	gSt.gameStarted = true
	printCleanBoard()
	redraw.Stop()
	redraw = time.NewTicker(time.Duration(gSt.tickerDelay) * time.Millisecond)
}

func handleGameOver() {
	gSt.gameStarted = false
	gSt.gameOver = true
	gSt.endTimestamp = time.Now().UTC().Unix()

	printGameSummary()
	if gSt.score > 0 {
		gEnv.isHighScoreRequest = true
		go postHighScore()
		for {
			if !gEnv.isHighScoreRequest {
				printCleanBoard()
				printHighScorePostResult(gEnv.highScoreRequestResult)
				return
			}
			time.Sleep(time.Duration(100) * time.Millisecond)
			printSpinner()
			termbox.Flush()
		}
	} else {
		printCleanBoard()
		printGameIntro()
		termbox.Flush()
		return
	}
}

func printHighScorePostResult(success bool) {
	var message string
	var bgColor termbox.Attribute
	if success {
		message = "  Your highscore has been posted.  "
		bgColor = termbox.ColorBlue
	} else {
		message = "  Error while posting your highscore :(  "
		bgColor = termbox.ColorRed
	}

	textLines := []string{
		" ",
		message,
		" ",
		"  Press space to continue.  ",
		"  ",
	}

	printCenteredTextWindow(textLines, termbox.ColorYellow, bgColor)
}

func postHighScore() {
	defer func() {
		gEnv.isHighScoreRequest = false
	}()

	highScoreData := highScore{
		PlayerName:     gEnv.playerName,
		Score:          gSt.score,
		EliMode:        gSt.eliMode,
		BoardWidth:     gEnv.boardWidth,
		BoardHeight:    gEnv.boardHeight,
		Fruits:         gSt.fruits,
		SpecialFruits:  gSt.specialFruits,
		TickerDelay:    int(gSt.tickerDelay),
		StartTimestamp: gSt.startTimestamp,
		EndTimestamp:   gSt.endTimestamp,
		AppVersion:     appVersion,
		Goos:           appPlatform,
	}

	highScoreJSON, err := json.Marshal(highScoreData)
	if err != nil {
		gEnv.highScoreRequestResult = false
		return
	}

	hmac := hmac.New(sha256.New, []byte(flc+ironsys))
	hmac.Write([]byte(highScoreJSON))
	iron := hex.EncodeToString(hmac.Sum(nil))

	highScoreWrapperData := highScoreWrapper{
		Signature: iron,
		Data:      highScoreData,
	}

	highScoreWrapperJSON, err := json.Marshal(highScoreWrapperData)
	if err != nil {
		gEnv.highScoreRequestResult = false
		return
	}

	req, err := http.NewRequest("POST", highScoresURL, bytes.NewBuffer(highScoreWrapperJSON))
	if err != nil {
		gEnv.highScoreRequestResult = false
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		gEnv.highScoreRequestResult = false
		return
	}
	defer resp.Body.Close()
	gEnv.highScoreRequestResult = resp.StatusCode == 201
}

func interruptPause() {
	if gSt.gamePaused {
		gSt.gamePaused = false
		printCleanBoard()
		if gSt.isFruit {
			printFruit(gSt.fruitCoords)
		}
	}
}

func printCleanBoard() {
	termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	printStatusBar()
	printFillerColumn()
}

// Checks if given coordinates pair is in given slice of coordinates pairs
func areCoordsInSlice(coordsPair coords, notAllowedCoords []coords) bool {
	for _, notAllowedCoordsPair := range notAllowedCoords {
		if coordsPair == notAllowedCoordsPair {
			return true
		}
	}

	return false
}

// Prints whole snake.
func printSnake(coords []coords) {
	for _, coordsPair := range coords {
		printSnakeSegment(coordsPair)
	}
}

// Erases whole snake.
func eraseSnake(coords []coords) {
	for _, coordsPair := range coords {
		eraseSnakeSegment(coordsPair)
	}
}

// Prints snakes segment at given coordinates.
func printSnakeSegment(coordsPair coords) {
	termbox.SetCell(coordsPair.x*2, coordsPair.y+1, rune(' '), termbox.ColorWhite, termbox.ColorWhite)
	termbox.SetCell(coordsPair.x*2+1, coordsPair.y+1, rune(' '), termbox.ColorWhite, termbox.ColorWhite)
}

// Erases single snakes segment at given coordinates.
func eraseSnakeSegment(coordsPair coords) {
	termbox.SetCell(coordsPair.x*2, coordsPair.y+1, rune(' '), termbox.ColorDefault, termbox.ColorDefault)
	termbox.SetCell(coordsPair.x*2+1, coordsPair.y+1, rune(' '), termbox.ColorDefault, termbox.ColorDefault)
}

// Prints "fruit" at given coordinates.
func printFruit(coordsPair coords) {
	color := termbox.ColorYellow
	if gSt.isSpecialFruit {
		color = termbox.ColorRed
	}
	termbox.SetCell(coordsPair.x*2, coordsPair.y+1, rune(' '), termbox.ColorDefault, color)
	termbox.SetCell(coordsPair.x*2+1, coordsPair.y+1, rune(' '), termbox.ColorDefault, color)
}

// Erases "fruit" at given coordinates.
func eraseFruit(coordsPair coords) {
	termbox.SetCell(coordsPair.x*2, coordsPair.y+1, rune(' '), termbox.ColorDefault, termbox.ColorDefault)
	termbox.SetCell(coordsPair.x*2+1, coordsPair.y+1, rune(' '), termbox.ColorDefault, termbox.ColorDefault)
}

// Prints status bar in the top row, containig game name, version and current score.
func printStatusBar() {
	soundState := "OFF"
	if gEnv.soundOn {
		soundState = "ON"
	}
	statusString := " " + appName + " ver. " + appVersion + " Sound: " + soundState
	if gSt.eliMode {
		statusString += " (Eli mode)"
	}
	scoreString := " Score: " + strconv.Itoa(gSt.score) + " "

	i := 0
	for _, char := range statusString {
		termbox.SetCell(i, 0, rune(char), termbox.ColorYellow, termbox.ColorBlue)
		i++
	}

	for i < gEnv.consoleWidth-len(scoreString) {
		termbox.SetCell(i, 0, rune(' '), termbox.ColorYellow, termbox.ColorBlue)
		i++
	}

	for _, char := range scoreString {
		termbox.SetCell(i, 0, rune(char), termbox.ColorYellow, termbox.ColorBlue)
		i++
	}
}

func printGamePausedInfo() {
	textLines := []string{
		" ",
		"  Game paused. Press arrow key to resume.  ",
		" ",
	}

	printCenteredTextWindow(textLines, termbox.ColorYellow, termbox.ColorBlue)
}

// Prints blank column on the right if console width is odd.
func printFillerColumn() {
	if gEnv.consoleWidth%2 != 0 {
		for i := 1; i < gEnv.consoleHeight; i++ {
			termbox.SetCell(gEnv.consoleWidth-1, i, rune(' '), termbox.ColorWhite, termbox.ColorBlue)
		}
	}
}

// Prints game introduction.
func printGameIntro() {
	gSt.gameOver = false
	textLines := []string{
		" ",
		"  " + appName + " ver. " + appVersion + "  ",
		" ",
		"  Press Ctrl + R to start normal mode.  ",
		"  Press Ctrl + E to start in Eli mode (go through walls).  ",
		"  Press Ctrl + H to see highscores.  ",
		" ",
		"  During game:  ",
		"  - press Ctrl + P to pause  ",
		"  - press Ctrl + S to toggle sound ON/OFF  ",
		"  - press Ctrl + Q to quit  ",
		" ",
		"  Find out more at " + host + "  ",
		" ",
	}

	textLength := 0

	for _, textLine := range textLines {
		if len(textLine) > textLength {
			textLength = len(textLine)
		}
	}

	gEnv.spinner.state = true
	gEnv.spinner.xPos = (gEnv.consoleWidth-textLength)/2 + len(textLines[5])
	gEnv.spinner.yPos = (gEnv.consoleHeight-len(textLines))/2 + 5
	gEnv.spinner.fgColor = termbox.ColorYellow
	gEnv.spinner.bgColor = termbox.ColorBlue

	printCenteredTextWindow(textLines, termbox.ColorYellow, termbox.ColorBlue)
}

// Prints game summary containing score.
func printGameSummary() {
	textLines := []string{
		" ",
		"  Game over! Your score: " + strconv.Itoa(gSt.score) + "  ",
		" ",
	}

	highScoreTextLines := []string{
		"  Enter your name and press enter to continue.  ",
		"  Or press Esc if your are too shy to post your score...  ",
		" ",
		" ",
		" ",
	}

	noHighScoreTextLines := []string{
		"  Press space to continue.  ",
		" ",
	}

	if gSt.score > 0 {
		textLines = append(textLines, highScoreTextLines...)
	} else {
		textLines = append(textLines, noHighScoreTextLines...)
	}

	printCenteredTextWindow(textLines, termbox.ColorYellow, termbox.ColorRed)

	if gSt.score > 0 {
		textLength := 0

		for _, textLine := range textLines {
			if len(textLine) > textLength {
				textLength = len(textLine)
			}
		}

		textXOffset := (gEnv.consoleWidth-textLength)/2 + 3
		textYOffset := (gEnv.consoleHeight-len(textLines))/2 + 6

		//print input field filled with recently used player name
		for i := 0; i <= 19; i++ {
			character := ' '
			if len(gEnv.playerName) > i {
				character = rune(gEnv.playerName[i])
			}
			termbox.SetCell(textXOffset+i, textYOffset, rune(character), termbox.ColorDefault, termbox.ColorDefault)
		}

		textXOffset += len(gEnv.playerName)
		termbox.SetCursor(textXOffset, textYOffset)
		termbox.Flush()

		validName := readPlayerName(textXOffset, textYOffset)

		termbox.HideCursor()
		if validName {
			gEnv.spinner.state = true
			gEnv.spinner.xPos = (gEnv.consoleWidth-textLength)/2 + 24
			gEnv.spinner.yPos = textYOffset
			gEnv.spinner.fgColor = termbox.ColorYellow
			gEnv.spinner.bgColor = termbox.ColorRed
		} else {
			gSt.score = 0
		}
	}
}

func readPlayerName(xPos int, yPos int) bool {
	for {
		ev := termbox.PollEvent()
		if ev.Type == termbox.EventKey {
			if ev.Ch == '_' || ev.Ch == '-' || (ev.Ch >= 'a' && ev.Ch <= 'z') || (ev.Ch >= 'A' && ev.Ch <= 'Z') || (ev.Ch >= '0' && ev.Ch <= '9') {
				if len(gEnv.playerName) < 20 {
					termbox.SetCell(xPos, yPos, rune(ev.Ch), termbox.ColorDefault, termbox.ColorDefault)
					gEnv.playerName += string(ev.Ch)
					xPos++
				}
			} else if ev.Key == termbox.KeyBackspace || ev.Key == termbox.KeyBackspace2 {
				if len(gEnv.playerName) > 0 {
					gEnv.playerName = gEnv.playerName[0 : len(gEnv.playerName)-1]
					xPos--
					termbox.SetCell(xPos, yPos, rune(' '), termbox.ColorDefault, termbox.ColorDefault)
				}
			} else if ev.Key == termbox.KeyEnter {
				if len(gEnv.playerName) > 0 && len(gEnv.playerName) <= 20 {
					return true
				}
			} else if ev.Key == termbox.KeyEsc {
				return false

			}
			termbox.SetCursor(xPos, yPos)
			termbox.Flush()
		}
	}
}

// Prints windows with lines of text in the middle of the `c`onsole.
func printCenteredTextWindow(textLines []string, fgColor termbox.Attribute, bgColor termbox.Attribute) {
	textLength := 0

	for _, textLine := range textLines {
		if len(textLine) > textLength {
			textLength = len(textLine)
		}
	}

	textXOffset := (gEnv.consoleWidth - textLength) / 2
	textYOffset := (gEnv.consoleHeight - len(textLines)) / 2

	for i := 0; i <= textLength+1; i++ {
		j := 0
		for index := range textLines {
			if i >= len(textLines[index]) {
				textLines[index] += " "
			}
			if i < textLength {
				termbox.SetCell(textXOffset+i, textYOffset+j, rune([]rune(textLines[index])[i]), fgColor, bgColor)
			} else if j > 0 {
				termbox.SetCell(textXOffset+i, textYOffset+j, rune(' '), termbox.ColorDefault, fgColor)
			}
			j++
		}

		if i > 1 {
			termbox.SetCell(textXOffset+i, textYOffset+len(textLines), rune(' '), termbox.ColorDefault, fgColor)
		}
	}
}

func printSpinner() {
	if gEnv.spinner.state {
		gEnv.spinner.counter++
		if gEnv.spinner.counter > 3 {
			gEnv.spinner.counter = 0
		}
		character := rune('\u2598')
		switch gEnv.spinner.counter {
		case 1:
			character = rune('\u259D')
		case 2:
			character = rune('\u2597')
		case 3:
			character = rune('\u2596')
		}

		termbox.SetCell(gEnv.spinner.xPos, gEnv.spinner.yPos, character, gEnv.spinner.fgColor, gEnv.spinner.bgColor)
	}
}

// Returns random snakes horizontal and vertical directions.
func generateRandomSnakeDirection() (int, int) {
	snakeDirX, snakeDirY := 0, -1

	// 4 possibilietes, case 0 handled above
	dirCase := randomGenerator.Intn(4)
	switch dirCase {
	case 1:
		snakeDirX, snakeDirY = 1, 0
	case 2:
		snakeDirX, snakeDirY = 0, 1
	case 3:
		snakeDirX, snakeDirY = -1, 0
	}

	return snakeDirX, snakeDirY
}

func showHighScores() {
	gEnv.isHighScoreRequest = true
	go fetchHighScores()
	for {
		if !gEnv.isHighScoreRequest {
			printCleanBoard()
			printHighScores()
			return
		}
		time.Sleep(time.Duration(100) * time.Millisecond)
		printSpinner()
		termbox.Flush()
	}
}

func printHighScores() {
	gSt.serverContentShown = true
	textLines := []string{
		"  ",
		"  Snakolek highscores:  ",
		"  ",
		fmt.Sprintf("%27v", "Player name") + fmt.Sprintf("%12v", "Score") + fmt.Sprintf("%5v", "Eli") + fmt.Sprintf("%12v", "Duration") + fmt.Sprintf("%18v", "Date"),
		"  -------------------------------------------------------------------------  ",
	}
	bgColor := termbox.ColorBlue

	i := 0

	if gEnv.highScoreRequestResult {
		for _, onlineHighScore := range onlineHighScores {
			i++
			textLines = append(textLines, onlineHighScoreToTextLine(i, onlineHighScore))
		}
	} else {
		textLines = []string{
			" ",
			"  Error while fetching highscores from server :(  ",
		}
		bgColor = termbox.ColorRed
	}

	textLines = append(textLines, []string{
		" ",
		"  More at " + host + "/high-scores  ",
		" ",
		"  Press space to continue.",
		" ",
	}...)

	printCenteredTextWindow(textLines, termbox.ColorYellow, bgColor)
	termbox.Flush()
}

func onlineHighScoreToTextLine(i int, onlineHighScore onlineHighScore) string {
	result := "  "
	result += fmt.Sprintf("%2v", strconv.Itoa(i)) + ".  "
	result += fmt.Sprintf("%20v", onlineHighScore.PlayerName) + "  "
	result += fmt.Sprintf("%10v", strconv.Itoa(onlineHighScore.Score)) + "  "

	eliModeString := "NO"
	if onlineHighScore.EliMode {
		eliModeString = "YES"
	}

	result += fmt.Sprintf("%3v", eliModeString) + "  "
	result += fmt.Sprintf("%10v", strconv.Itoa(onlineHighScore.Duration)) + "  "

	dateTime, err := time.Parse(time.RFC3339, onlineHighScore.CreatedAt)

	if err != nil {
		return " "
	}

	result += dateTime.Format("2006-01-02 15:04")
	result += "  "

	return result
}

func fetchHighScores() {
	defer func() {
		gEnv.isHighScoreRequest = false
	}()

	resp, err := http.Get(highScoresURL + "?limit=10")
	if err != nil {
		gEnv.highScoreRequestResult = false
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		gEnv.highScoreRequestResult = false
		return
	}

	json.Unmarshal(body, &onlineHighScores)

	gEnv.highScoreRequestResult = resp.StatusCode == 200
}

func printServerMessages() bool {
	onlineMessages := []onlineMessage{}

	q := url.Values{}
	q.Add(url.QueryEscape("app_version"), url.QueryEscape(appVersion))
	q.Add(url.QueryEscape("goos"), url.QueryEscape(appPlatform))

	resp, err := http.Get(messagesURL + "?" + q.Encode())
	if err != nil {
		return false
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	json.Unmarshal(body, &onlineMessages)

	if len(onlineMessages) > 0 {
		textLines := []string{
			" ",
			"  News from Snakolek:  ",
		}

		for _, message := range onlineMessages {
			dateTime, err := time.Parse(time.RFC3339, message.CreatedAt)

			if err != nil {
				return false
			}

			textLine := dateTime.Format("2006-01-02 15:04") + "  " + message.Content
			textLines = append(textLines, " ")
			textLines = append(textLines, "  "+textLine+"  ")
			textLines = append(textLines, " ")
		}

		textLines = append(textLines, " ")
		textLines = append(textLines, "  Press space to continue.  ")
		textLines = append(textLines, " ")

		printCenteredTextWindow(textLines, termbox.ColorYellow, termbox.ColorBlue)
		gSt.serverContentShown = true

		return true
	}

	return false
}

// Makes a beep (sound) if sound is globally turned on
func beep() {
	if gEnv.soundOn {
		os.Stdout.Write([]byte("\u0007"))
	}
}
