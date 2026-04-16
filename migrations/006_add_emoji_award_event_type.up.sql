ALTER TABLE events DROP CONSTRAINT events_event_type_check,
              ADD CONSTRAINT events_event_type_check
                  CHECK (event_type IN ('push', 'mr_opened', 'mr_merged', 'mr_draft', 'emoji_award', 'unrecognized'));
