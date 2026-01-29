-- 005_cluster_user_association.sql
-- Associate clusters with the users who created them

-- Add user_id column to clusters table
ALTER TABLE clusters ADD COLUMN IF NOT EXISTS user_id INT REFERENCES users(id) ON DELETE SET NULL;

-- Create index for efficient user-based queries
CREATE INDEX IF NOT EXISTS idx_clusters_user_id ON clusters(user_id);

-- Update existing clusters to have NULL user_id (no owner)
-- New clusters will have the creating user's ID set
