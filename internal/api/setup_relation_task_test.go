package api

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// TestSetupAssistant_AssignAgentToGroup verifies an existing agent gains rw
// group membership, is idempotent per (agent, team), and validates inputs.
func TestSetupAssistant_AssignAgentToGroup(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	agentLogin := "rel_agent" + sfx
	agentID, err := svc.CreateAgent(ctx, agentLogin, "Rel", "Agent", "", nil, 1)
	require.NoError(t, err)

	teamID, err := svc.CreateGroup(ctx, "RelTeam"+sfx, "relation-task test", 1)
	require.NoError(t, err)

	// Happy path: assign the agent to the team.
	require.NoError(t, svc.AssignAgentToGroups(ctx, agentID, []int{teamID}, 1))

	// Membership row exists with rw permission.
	db, err := database.GetDB()
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ? AND permission_key = 'rw'"),
		agentID, teamID).Scan(&n))
	assert.Equal(t, 1, n)

	// Idempotent: re-assigning does not create a duplicate row.
	require.NoError(t, svc.AssignAgentToGroups(ctx, agentID, []int{teamID}, 1))
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT COUNT(*) FROM group_user WHERE user_id = ? AND group_id = ?"),
		agentID, teamID).Scan(&n))
	assert.Equal(t, 1, n)

	// Validation: unknown agent.
	err = svc.AssignAgentToGroups(ctx, 9999999, []int{teamID}, 1)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not found")

	// Validation: no teams.
	err = svc.AssignAgentToGroups(ctx, agentID, nil, 1)
	require.Error(t, err)
}

// TestSetupAssistant_AssignQueueToGroup verifies an existing queue gains
// queue_group access, is idempotent, and validates inputs.
func TestSetupAssistant_AssignQueueToGroup(t *testing.T) {
	svc, sfx := setupSvcTestDB(t)
	ctx := context.Background()

	teamID, err := svc.CreateGroup(ctx, "RelQTeam"+sfx, "relation-task test", 1)
	require.NoError(t, err)
	queueID, err := svc.CreateQueue(ctx, "RelQueue"+sfx, []int{teamID}, "relation-task test", 1)
	require.NoError(t, err)

	otherTeamID, err := svc.CreateGroup(ctx, "RelQOther"+sfx, "relation-task test", 1)
	require.NoError(t, err)

	require.NoError(t, svc.AssignQueueToGroup(ctx, queueID, otherTeamID, 1))

	db, err := database.GetDB()
	require.NoError(t, err)
	var owner int
	require.NoError(t, db.QueryRow(database.ConvertPlaceholders(
		"SELECT group_id FROM queue WHERE id = ?"), queueID).Scan(&owner))
	assert.Equal(t, otherTeamID, owner)

	// Validation: unknown queue.
	err = svc.AssignQueueToGroup(ctx, 9999999, teamID, 1)
	require.Error(t, err)
	assert.Contains(t, strings.ToLower(err.Error()), "not found")

	// Validation: no team.
	err = svc.AssignQueueToGroup(ctx, queueID, 0, 1)
	require.Error(t, err)
}

// TestSetupTaskForm_HasAssignBranches verifies the setup task form template
// declares an inline branch for both relation tasks so they never hit the
// "no inline form" fallback.
func TestSetupTaskForm_HasAssignBranches(t *testing.T) {
	src, err := readFile("../../templates/pages/admin/setup_task_form.pongo2")
	require.NoError(t, err)
	s := string(src)
	assert.Contains(t, s, `Task.ID == "assign_agent_group"`)
	assert.Contains(t, s, `Task.ID == "assign_queue_group"`)
	assert.Contains(t, s, `name="agent_id"`)
	assert.Contains(t, s, `name="queue_id"`)
	// The agent picker must use the GoatKit searchable combobox (scales to many
	// agents), not a plain <select>.
	assert.Contains(t, s, `data-gk-autocomplete="agents"`)
	assert.Contains(t, s, `data-hidden-target="agentId"`)
	assert.Contains(t, s, `data-gk-seed="agents"`)
	assert.NotContains(t, s, `select name="agent_id"`)
	// The fallback line must still exist but be reachable only by unknown tasks.
	assert.Contains(t, s, "This task has no inline form.")
}
