package cmdHandlers

import (
	"context"
	"fmt"
	"github.com/ilinovom/summary-tasks-bot/pkg/telegram"
	"log"
	"strings"
)

// handleStopCommand processes the /stop command.
// It disables scheduled news for the user.
func (c *CmdHandler) handleStopCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /stop", m.Chat.ID, m.Chat.Username)
	if err := c.userService.Stop(ctx, m.Chat.ID); err != nil {
		log.Println("stop:", err)
	} else {
		c.sendMessage(ctx, m.Chat.ID, c.messages["stopped"], nil)
	}
}

// handleInfoCommand sends the list of available commands.
func (c *CmdHandler) handleInfoCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /info", m.Chat.ID, m.Chat.Username)
	c.sendLongMessage(ctx, m.Chat.ID, c.messages["info"])
}

// handleTariffsCommand prints information about available tariffs.
func (c *CmdHandler) handleTariffsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /tariffs", m.Chat.ID, m.Chat.Username)
	c.sendLongMessage(ctx, m.Chat.ID, c.messages["tariffs"])
}

// handleMyTopicsCommand displays user's current topics.
func (c *CmdHandler) handleMyTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /my_topics", m.Chat.ID, m.Chat.Username)
	settings, err := c.repo.Get(ctx, m.Chat.ID)
	if err != nil {
		c.sendMessage(ctx, m.Chat.ID, c.messages["start_first"], nil)
		return
	}
	parts := []string{}
	for cat, types := range settings.Topics {
		parts = append(parts, fmt.Sprintf("%s: %s", cat, strings.Join(types, ", ")))
	}
	msg := fmt.Sprintf(c.messages["your_topics"], strings.Join(parts, "\n\n"))
	c.sendMessage(ctx, m.Chat.ID, msg, nil)
}

// handleTopicsCommand shows the topics submenu.
func (c *CmdHandler) handleTopicsCommand(ctx context.Context, m *telegram.Message) {
	log.Printf("user %d(@%s) called /topics", m.Chat.ID, m.Chat.Username)
	c.sendMessage(ctx, m.Chat.ID, c.messages["topics_menu"], nil)
}
