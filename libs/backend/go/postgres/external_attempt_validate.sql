DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'external_attempt'::regclass AND conname = 'external_attempt_entries_check' AND NOT convalidated) THEN
    ALTER TABLE external_attempt VALIDATE CONSTRAINT external_attempt_entries_check;
  END IF;
END
$$;
