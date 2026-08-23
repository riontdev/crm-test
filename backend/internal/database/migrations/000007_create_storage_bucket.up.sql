-- Create storage bucket for message attachments
INSERT INTO storage.buckets (id, name, public) VALUES ('attachments', 'attachments', true)
ON CONFLICT (id) DO NOTHING;

-- Allow all uploads (authenticated or anon)
CREATE POLICY "Allow all uploads" ON storage.objects
  FOR INSERT
  WITH CHECK (bucket_id = 'attachments');

-- Allow public reads
CREATE POLICY "Allow public reads" ON storage.objects
  FOR SELECT
  USING (bucket_id = 'attachments');
