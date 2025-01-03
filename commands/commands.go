package commands

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"arono/util"
)

func Ping(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) {
	s.ChannelMessageSend(m.ChannelID, "pong")
}

func Help(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) {
	fileName := "vinner.jpg"

	f, err := os.Open(fileName)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	ms := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title: "Help menu",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Codeforces Tictactoe",
					Value:  "start\nupdate\nend",
					Inline: true,
				},
			},
			Image: &discordgo.MessageEmbedImage{
				URL: "attachment://" + fileName,
			},
		},
		Files: []*discordgo.File{
			{
				Name:   fileName,
				Reader: f,
			},
		},
	}

	s.ChannelMessageSendComplex(m.ChannelID, ms)
}

func Challenge(s *discordgo.Session, m *discordgo.MessageCreate, args []string, duelMap *util.DuelMap, challengeMap *util.ChallengeMap) {
	if len(args) < 3 {
		s.ChannelMessageSend(m.ChannelID, "Missing an opponent")
		return
	}
	// checking for the validity of the ID (arg 0)
	if len(args[0]) < 4 {
		s.ChannelMessageSend(m.ChannelID, args[0]+" is not a valid user")
		return
	}

	if _, err := strconv.Atoi(args[0][2 : len(args[0])-1]); err != nil {
		s.ChannelMessageSend(m.ChannelID, args[0]+" is not a valid user")
		return
	}
	opponent := args[0][2 : len(args[0])-1]

	// checking if anyone is already in a duel
	duelMap.RWMutex.RLock()
	_, duelOccupied := duelMap.Map[m.Author.ID]
	_, opponentDuelOccupied := duelMap.Map[opponent]
	duelMap.RWMutex.RUnlock()

	if duelOccupied {
		s.ChannelMessageSend(m.ChannelID, "You are already in another duel!")
		return
	}
	if opponentDuelOccupied {
		s.ChannelMessageSend(m.ChannelID, "<@"+opponent+"> is already in another duel!")
		return
	}

	// checking if anyone is challenged
	challengeMap.RWMutex.RLock()
	_, challengeOccupied := challengeMap.Map[m.Author.ID]
	_, opponentChallengeOccupied := challengeMap.Map[opponent]
	challengeMap.RWMutex.RUnlock()

	if challengeOccupied {
		s.ChannelMessageSend(m.ChannelID, "You are already challenged!")
		return
	}
	if opponentChallengeOccupied {
		s.ChannelMessageSend(m.ChannelID, "<@"+opponent+">is already challenged by someone else!")
		return
	}

	// checking if codeforces handles exists
	if !util.UserExists([]string{args[1], args[2]}) {
		s.ChannelMessageSend(m.ChannelID, "Codeforces handle doesn't exist")
		return
	}

	// adding to the challenging map
	challengeMap.RWMutex.Lock()
	challengeMap.Map[m.Author.ID] = append([]string{opponent}, args[1:]...)
	challengeMap.Map[opponent] = append([]string{"!" + m.Author.ID}, args[1:]...)
	challengeMap.RWMutex.Unlock()

	s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> is challenging <@"+opponent+">. Type `~accept` to accept the challenge")

	time.Sleep(20 * time.Second)

	// the timer expired, check if they are still in challengeMap (not accepted), or they are not (accepted and removed)

	challengeMap.RWMutex.Lock()
	defer challengeMap.RWMutex.Unlock()
	_, exists := challengeMap.Map[m.Author.ID]
	if !exists {
		return
	}
	delete(challengeMap.Map, m.Author.ID)
	delete(challengeMap.Map, opponent)

	s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> Your challenge to <@"+opponent+"> has expired!")
}

func Accept(s *discordgo.Session, m *discordgo.MessageCreate, args []string, duelMap *util.DuelMap, challengeMap *util.ChallengeMap) {
	challengeMap.RWMutex.RLock()
	challenger, occupied := challengeMap.Map[m.Author.ID]
	challengeMap.RWMutex.RUnlock()

	if !occupied || !strings.HasPrefix(challenger[0], "!") {
		s.ChannelMessageSend(m.ChannelID, "You are not challenged by anyone")
		return
	}

	opponent := challenger[0][1:]

	challengeMap.RWMutex.Lock()
	delete(challengeMap.Map, m.Author.ID)
	delete(challengeMap.Map, opponent)
	challengeMap.RWMutex.Unlock()

	s.ChannelMessageSend(m.ChannelID, "<@"+m.Author.ID+"> has accepted a challenge from <@"+opponent+">")

	fmt.Println(challenger, len(challenger))

	var rating float64 = 0
	if len(challenger) > 3 {
		ratingTmp, err := strconv.ParseFloat(challenger[3], 64)
		if err != nil {
			s.ChannelMessageSend(m.ChannelID, challenger[3]+" is not a valid rating")
			return
		}
		rating = ratingTmp
	}

	var includeTags, excludeTags []string
	if len(challenger) > 4 {
		var ok bool
		includeTags, excludeTags, ok = util.Parse(challenger[4:])
		if !ok {
			s.ChannelMessageSend(m.ChannelID, "Invalid tag format")
			return
		}

		if !util.TagSubset(includeTags) || !util.TagSubset(excludeTags) {
			s.ChannelMessageSend(m.ChannelID, "Tag error; Some of the tags don't exist")
			return
		}
	}

	submission1 := util.GetAttemptedSubmissions(challenger[1])
	submission2 := util.GetAttemptedSubmissions(challenger[2])

	excludeProblems := make(map[string]int)
	for k := range submission1 {
		excludeProblems[k] = 0
	}
	for k := range submission2 {
		excludeProblems[k] = 0
	}

	problems, ok := util.RandomSample(util.GetProblems(rating, includeTags, excludeTags, excludeProblems), 9)
	// problems = append(problems, util.Problem{
	// 	ID: "938/A",
	// })

	if !ok {
		s.ChannelMessageSend(m.ChannelID, "Not enough problems for this criteria")
		return
	}

	duelMap.RWMutex.Lock()
	duelMap.Map[opponent] = util.GameState{
		Duelists: []string{opponent, m.Author.ID},
		Handles:  challenger[1:3],
		Problems: problems,
	}
	duelMap.Map[m.Author.ID] = util.GameState{
		Duelists: []string{opponent, m.Author.ID},
		Handles:  challenger[1:3],
		Problems: problems,
	}
	fmt.Println(duelMap.Map)
	duelMap.RWMutex.Unlock()

	// printing the embed
	// fileName := "vinner.jpg"

	// f, err := os.Open(fileName)
	// if err != nil {
	// 	panic(err)
	// }
	// defer f.Close()

	problemsLinks := ""
	for i, p := range problems {
		problemsLinks += "[" + strconv.Itoa(i+1) + "](https://codeforces.com/problemset/problem/" + p.ID + ")  "
	}

	ms := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title: "TicTacToe",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Type `~update` to update game state",
					Value:  problemsLinks,
					Inline: true,
				},
				{
					Name:   "",
					Value:  "``` 1 | 2 | 3 \n-----------\n 4 | 5 | 6 \n-----------\n 7 | 8 | 9 \n```",
					Inline: false,
				},
			},
		},
	}

	s.ChannelMessageSendComplex(m.ChannelID, ms)
}

func End(s *discordgo.Session, m *discordgo.MessageCreate, args []string, duelMap *util.DuelMap, challengeMap *util.ChallengeMap) {
	challengeMap.RWMutex.RLock()
	challenger, occupied := challengeMap.Map[m.Author.ID]
	challengeMap.RWMutex.RUnlock()

	if occupied {
		opponent := challenger[0][1:]

		challengeMap.RWMutex.Lock()
		delete(challengeMap.Map, m.Author.ID)
		delete(challengeMap.Map, opponent)
		challengeMap.RWMutex.Unlock()

		s.ChannelMessageSend(m.ChannelID, "Challenge rejected")
	}

	duelMap.RWMutex.RLock()
	state, duelOccupied := duelMap.Map[m.Author.ID]
	duelMap.RWMutex.RUnlock()

	if duelOccupied {
		opponent := state.Duelists[0]
		if opponent == m.Author.ID {
			opponent = state.Duelists[1]
		}

		duelMap.RWMutex.Lock()
		delete(duelMap.Map, m.Author.ID)
		delete(duelMap.Map, opponent)
		duelMap.RWMutex.Unlock()

		s.ChannelMessageSend(m.ChannelID, "The dual between <@"+m.Author.ID+"> and <@"+opponent+"> has been ended")
	}

	if !occupied && !duelOccupied {
		s.ChannelMessageSend(m.ChannelID, "You are neither being challenged nor in a duel.")
	}
}

func Update(s *discordgo.Session, m *discordgo.MessageCreate, args []string, duelMap *util.DuelMap, challengeMap *util.ChallengeMap) {
	duelMap.RWMutex.RLock()
	state, duelOccupied := duelMap.Map[m.Author.ID]
	duelMap.RWMutex.RUnlock()

	if !duelOccupied {
		s.ChannelMessageSend(m.ChannelID, "You are not currently in a duel.")
		return
	}
	s.ChannelMessageSend(m.ChannelID, "Updating...")

	fmt.Println("getting submissions of:", state.Handles)

	submission1 := util.GetSuccesfulSubmissions(state.Handles[0])
	time.Sleep(3 * time.Second)
	submission2 := util.GetSuccesfulSubmissions(state.Handles[1])

	changes := false

	for problemID, sub := range submission1 {
		for i, gameProblem := range state.Problems {
			if problemID == gameProblem.ID {
				if gameProblem.Timestamp == 0 || gameProblem.Timestamp > sub.Timestamp {
					state.Problems[i].Solver = 1
					state.Problems[i].Timestamp = sub.Timestamp

					changes = true
				}
			}
		}
	}

	for problemID, sub := range submission2 {
		for i, gameProblem := range state.Problems {
			if problemID == gameProblem.ID {
				if gameProblem.Timestamp == 0 || gameProblem.Timestamp > sub.Timestamp {
					state.Problems[i].Solver = 2
					state.Problems[i].Timestamp = sub.Timestamp

					changes = true
				}
			}
		}
	}

	duelMap.RWMutex.Lock()
	duelMap.Map[state.Duelists[0]] = state
	duelMap.Map[state.Duelists[1]] = state
	duelMap.RWMutex.Unlock()

	// resend the message with updated ttt board
	problemsLinks := ""
	board := "```"
	intBoard := []int{}
	for i, p := range state.Problems {
		problemsLinks += "[" + strconv.Itoa(i+1) + "](https://codeforces.com/problemset/problem/" + p.ID + ")  "
		intBoard = append(intBoard, p.Solver)

		board += " "
		switch p.Solver {
		case 0:
			board += strconv.Itoa(i + 1)
		case 1:
			board += "X"
		case 2:
			board += "O"
		}

		if i%3 != 2 {
			board += " |"
		} else {
			if i != 8 {
				board += "\n------------"
			}
			board += "\n"
		}
	}
	board += "```"

	ms := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title: "TicTacToe",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Type `~update` to update game state",
					Value:  problemsLinks,
					Inline: true,
				},
				{
					Name:   "",
					Value:  board,
					Inline: false,
				},
			},
		},
	}

	s.ChannelMessageSendComplex(m.ChannelID, ms)
	if !changes {
		s.ChannelMessageSend(m.ChannelID, "Nothing changed so far")
	}

	win1, win2 := util.IsGameOver(intBoard)
	fmt.Println(win1, win2)
	if win1 && win2 {
		s.ChannelMessageSend(m.ChannelID, "It's a draw!")
	} else if win1 {
		s.ChannelMessageSend(m.ChannelID, "<@"+state.Duelists[0]+"> wins!")
	} else if win2 {
		s.ChannelMessageSend(m.ChannelID, "<@"+state.Duelists[1]+"> wins!")
	}

	if win1 || win2 {
		End(s, m, nil, duelMap, challengeMap)
	}

	duelMap.RWMutex.RLock()
	fmt.Println("state now:", duelMap.Map[m.Author.ID].Problems)
	duelMap.RWMutex.RUnlock()
}
