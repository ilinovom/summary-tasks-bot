package cmdHandlers

import (
	"context"
	"github.com/ilinovom/summary-tasks-bot/internal/model"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"strings"
)

type convStage int

const (
	// StartCmd
	StageStartWelcome convStage = iota + 1
	StageStartChooseCategoryCount
	StageStartCategory
	StageStartCustomCategory
	StageStartInfoTypes

	// UpdateTopicsCmd
	StageUpdateTopicsChoice
	StageUpdateTopicsSelectManyExisting
	StageUpdateTopicsCategory
	StageUpdateTopicsCustomCategory
	StageUpdateTopicsInfoTypes

	// AddTopicsCmd
	//StageAddTopicsUpdateChoice
	StageAddTopicsCustomCategory
	StageAddTopicsCategory
	StageAddTopicsInfoTypes
	StageAddTopicsAddMore

	// DeleteTopicsCmd
	StageDeleteTopicsChoice
	StageDeleteTopicsSelect

	StageCustomCategory
	StageGetNewsCategory
	stageGetLast24hCategory
	stageSetTariffUser
	stageSetTariffChoice
)

type ConversationState struct {
	Cmd                 string
	Stage               convStage
	PrevStage           convStage
	Step                int
	CurrentCat          string
	OldCat              string
	Topics              map[string][]string
	CategoryLimit       int
	InfoLimit           int
	LastMsgID           int
	AvailableCats       []string
	Settings            *model.UserSettings
	AllowCustomCategory bool
	SelectedInfos       []string
	SelectedCats        []string
	TargetUser          string
	NewTariff           string
}

// setStage updates the conversation state and remembers the previous stage to
// support navigating backward.
func (cs *ConversationState) setStage(s convStage) {
	cs.PrevStage = cs.Stage
	cs.Stage = s
}

// back returns the conversation to the previous stage if possible.
func (cs *ConversationState) back() {
	if cs.PrevStage != 0 {
		cs.Stage = cs.PrevStage
		cs.PrevStage = 0
	}
}

// continueConversation processes messages that are part of a multi-step dialog
// and advances the conversation state machine accordingly.
func (c *CmdHandler) continueConversation(ctx context.Context, m *telegram.Message, cs *ConversationState) {
	if strings.EqualFold(m.Text, "Отмена") {
		c.deleteCurrentAndLastMsg(ctx, m.Chat.ID, m.MessageID)
		c.sendMessage(ctx, m.Chat.ID, c.messages["cancelled"], nil)
		delete(c.convs, m.Chat.ID)
		return
	}

	switch cs.Cmd {
	case StartCmd:
		c.continueStartFlow(ctx, m, cs)
	case UpdateTopicsCmd:
		c.continueUpdateFlow(ctx, m, cs)
	case AddTopicsCmd:
		c.continueAddFlow(ctx, m, cs)
	case DeleteTopicsCmd:
		c.continueDeleteFlow(ctx, m, cs)
	case GetNewsNowCmd:
		c.continueNewsFlow(ctx, m, cs)
	case GetLast24hNewsCmd:
		c.continueLast24hFlow(ctx, m, cs)
	case SettCmd:
		c.continueSetTariffFlow(ctx, m, cs)
	}
}
