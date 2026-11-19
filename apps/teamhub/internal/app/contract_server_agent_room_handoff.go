package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/simplexai-labs/linger/apps/teamhub/internal/agentauth"
	"github.com/simplexai-labs/linger/apps/teamhub/internal/agentroomhandoff"
	teamhubagentapi "github.com/simplexai-labs/linger/contracts-go/teamhubagentapi"
)

const agentRoomHandoffRequestMaxBytes = 64 << 10

type agentRoomHandoffService interface {
	HandOff(context.Context, string, agentroomhandoff.Input) (agentroomhandoff.Result, error)
}

func (a *App) handOffAgentRoomWork(c *gin.Context) {
	token, ok := a.agentCommandBearer(c)
	if !ok {
		return
	}
	var request teamhubagentapi.AgentRoomHandoffRequest
	body := http.MaxBytesReader(c.Writer, c.Request.Body, agentRoomHandoffRequestMaxBytes)
	if err := decodeStrictJSON(body, &request, "request body must contain one JSON object"); err != nil {
		a.writeAgentAPIError(c, http.StatusBadRequest, teamhubagentapi.AgentApiErrorCodeInvalidRequest, "Room handoff request is invalid")
		return
	}
	result, err := a.agentRoomHandoffs.HandOff(c.Request.Context(), token, agentroomhandoff.Input{
		InvocationID: request.InvocationId, RoomID: request.RoomId, Instruction: request.Instruction,
	})
	if a.writeAgentRoomHandoffError(c, err) {
		return
	}
	c.JSON(http.StatusOK, agentMessageReceipt(gin.H{"accepted": true, "inbox_id": result.InboxID, "handoff_id": result.HandoffID}, result.Replay,
		"This operation was already completed. The original handoff is returned; nothing new was posted."))
}

func (a *App) writeAgentRoomHandoffError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, agentroomhandoff.ErrInvalid), errors.Is(err, agentauth.ErrInvalid):
		a.writeAgentAPIError(c, http.StatusBadRequest, teamhubagentapi.AgentApiErrorCodeInvalidRequest, "Room handoff request is invalid")
	case errors.Is(err, agentauth.ErrCredentialInvalid):
		a.writeAgentAPIError(c, http.StatusUnauthorized, teamhubagentapi.AgentApiErrorCodeAgentCredentialInvalid, "Agent credential is invalid or expired")
	case errors.Is(err, agentroomhandoff.ErrNotRequestedByHuman):
		a.writeAgentAPIError(c, http.StatusForbidden, teamhubagentapi.AgentApiErrorCodeOperationForbidden,
			"Only a request your Human sent you in your direct chat can be handed to another Room; this turn did not start from one.")
	case errors.Is(err, agentroomhandoff.ErrNotFromDirectChat):
		a.writeAgentAPIError(c, http.StatusForbidden, teamhubagentapi.AgentApiErrorCodeOperationForbidden,
			"Only Another Me hands work to another Room, from its direct chat with its Human.")
	case errors.Is(err, agentauth.ErrForbidden):
		a.writeAgentAPIError(c, http.StatusForbidden, teamhubagentapi.AgentApiErrorCodeOperationForbidden, agentForbiddenMessage(err, "hand_off_to_room is not allowed for this Context"))
	case errors.Is(err, agentroomhandoff.ErrRoomUnavailable):
		a.writeAgentAPIError(c, http.StatusForbidden, teamhubagentapi.AgentApiErrorCodeForbidden,
			"That Room is unavailable: it must be another live Room where both you and your Human are members.")
	case errors.Is(err, agentroomhandoff.ErrIdempotencyConflict):
		a.writeAgentAPIError(c, http.StatusConflict, teamhubagentapi.AgentApiErrorCodeIdempotencyConflict, "Room handoff invocation was reused with different input")
	case errors.Is(err, agentroomhandoff.ErrUnavailable):
		a.writeAgentAPIError(c, http.StatusServiceUnavailable, teamhubagentapi.AgentApiErrorCodeDependencyUnavailable, "Room handoff service is unavailable")
	default:
		if a.logger != nil {
			a.logger.Error("agent.room_handoff_failed", "request_id", c.GetString("request_id"), "error_type", fmt.Sprintf("%T", err))
		}
		a.writeAgentAPIError(c, http.StatusInternalServerError, teamhubagentapi.AgentApiErrorCodeInternalError, "Room handoff service could not complete the request")
	}
	return true
}
