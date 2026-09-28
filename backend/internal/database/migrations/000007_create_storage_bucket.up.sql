-- Supabase-only: create the storage bucket and policies for message attachments.
-- On a plain local Postgres the storage schema does not exist, so this is a no-op.

DO $do$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'storage') THEN
    INSERT INTO storage.buckets (id, name, public) VALUES ('attachments', 'attachments', true)
    ON CONFLICT (id) DO NOTHING;

    CREATE POLICY "Allow all uploads" ON storage.objects
      FOR INSERT
      WITH CHECK (bucket_id = 'attachments');

    CREATE POLICY "Allow public reads" ON storage.objects
      FOR SELECT
      USING (bucket_id = 'attachments');
  END IF;
END
$do$;