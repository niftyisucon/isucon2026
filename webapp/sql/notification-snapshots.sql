-- Apply before starting the new readers; existing notifications are left intact.
ALTER TABLE posting_windows ADD COLUMN IF NOT EXISTS notification_user_ids text[],
                            ADD COLUMN IF NOT EXISTS notification_ids text[];
