-- Add color column to ticket_priority if missing (added in Znuny 6.5.1)
ALTER TABLE ticket_priority ADD COLUMN IF NOT EXISTS color VARCHAR(25) NOT NULL DEFAULT '#cdcdcdff';

-- Add color column to ticket_state if missing (added in Znuny 6.5.1)
ALTER TABLE ticket_state ADD COLUMN IF NOT EXISTS color VARCHAR(25) NOT NULL DEFAULT '#8D8D9BFF';

-- Set default colors for standard priorities (matching Znuny initial_insert values)
UPDATE ticket_priority SET color = '#03c4f0ff' WHERE id = 1 AND color = '#cdcdcdff';
UPDATE ticket_priority SET color = '#83bfc8ff' WHERE id = 2 AND color = '#cdcdcdff';
UPDATE ticket_priority SET color = '#cdcdcdff' WHERE id = 3 AND color = '#cdcdcdff';
UPDATE ticket_priority SET color = '#ffaaaaff' WHERE id = 4 AND color = '#cdcdcdff';
UPDATE ticket_priority SET color = '#ff505eff' WHERE id = 5 AND color = '#cdcdcdff';

-- Set default colors for standard states (matching Znuny initial_insert values)
UPDATE ticket_state SET color = '#50B5FFFF' WHERE id = 1 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#3DD598FF' WHERE id = 2 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#FC5A5AFF' WHERE id = 3 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#FFC542FF' WHERE id = 4 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#8D8D9BFF' WHERE id = 5 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#FFC542FF' WHERE id = 6 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#83BFC8FF' WHERE id = 7 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#83BFC8FF' WHERE id = 8 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#FF9C3EFF' WHERE id = 9 AND color = '#8D8D9BFF';
UPDATE ticket_state SET color = '#FF9C3EFF' WHERE id = 10 AND color = '#8D8D9BFF';
