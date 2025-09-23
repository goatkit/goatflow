-- Add standard configuration queue for OTRS compatibility
-- This queue is essential for proper OTRS operations and is expected by many OTRS modules

-- Check if standard configuration queue already exists, if not create it
-- The actual queue name is configurable via environment variable
INSERT INTO queue (name, group_id, unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id, create_by, change_by, create_time, change_time)
SELECT 
    'Configuration' as name,
    2 as group_id,  -- Using 'users' group (id=2) 
    0 as unlock_timeout,
    1 as follow_up_id,
    0 as follow_up_lock,
    'Standard configuration queue for OTRS compatibility' as comments,
    1 as valid_id,
    1 as create_by,
    1 as change_by,
    NOW() as create_time,
    NOW() as change_time
WHERE NOT EXISTS (
    SELECT 1 FROM queue WHERE name = 'Configuration'
);

-- Also ensure we have standard OTRS queues if they're missing
INSERT INTO queue (name, group_id, unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id, create_by, change_by, create_time, change_time)
SELECT * FROM (VALUES
    ('IT Support', 2, 0, 1, 0, 'IT Support queue', 1, 1, 1, NOW(), NOW()),
    ('HR', 2, 0, 1, 0, 'Human Resources queue', 1, 1, 1, NOW(), NOW()),
    ('Sales', 2, 0, 1, 0, 'Sales department queue', 1, 1, 1, NOW(), NOW()),
    ('Finance', 2, 0, 1, 0, 'Finance department queue', 1, 1, 1, NOW(), NOW())
) AS t(name, group_id, unlock_timeout, follow_up_id, follow_up_lock, comments, valid_id, create_by, change_by, create_time, change_time)
WHERE NOT EXISTS (
    SELECT 1 FROM queue WHERE name = t.name
);

-- Update existing queues to use the correct group (users group id=2)
UPDATE queue SET group_id = 2 WHERE group_id = 1;