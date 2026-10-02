package api

import (
	"database/sql"

	"github.com/goatkit/goatflow/internal/platform/database"
)

// userGroupPermission is one group an agent belongs to, with the OTRS
// permission keys (ro, move_into, create, note, owner, priority, rw) the agent
// holds in it. group_user stores one row per key.
type userGroupPermission struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Permissions []string `json:"permissions"`
}

// loadUserGroupPermissions returns the agent's direct group memberships from
// group_user, ordered by group name.
func loadUserGroupPermissions(db *sql.DB, userID int) ([]userGroupPermission, error) {
	rows, err := db.Query(database.ConvertPlaceholders(`
		SELECT g.id, g.name, gu.permission_key
		FROM group_user gu
		INNER JOIN groups g ON g.id = gu.group_id
		WHERE gu.user_id = ?
		ORDER BY g.name, gu.permission_key
	`), userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []userGroupPermission{}
	for rows.Next() {
		var id int
		var name, key string
		if err := rows.Scan(&id, &name, &key); err != nil {
			return nil, err
		}
		if n := len(groups); n > 0 && groups[n-1].ID == id {
			perms := groups[n-1].Permissions
			if perms[len(perms)-1] != key { // group_user has no unique key; collapse duplicates
				groups[n-1].Permissions = append(perms, key)
			}
			continue
		}
		groups = append(groups, userGroupPermission{ID: id, Name: name, Permissions: []string{key}})
	}
	return groups, rows.Err()
}
