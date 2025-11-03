-- Create team members table
CREATE TABLE IF NOT EXISTS team_members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    department VARCHAR(100) NOT NULL,
    position VARCHAR(100) NOT NULL,
    bio TEXT,
    skills JSONB, -- JSON array
    linkedin VARCHAR(255),
    twitter VARCHAR(255),
    github VARCHAR(255),
    visibility VARCHAR(50) NOT NULL DEFAULT 'internal', -- public, internal, private
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP,
    UNIQUE(user_id)
);

-- Create indexes
CREATE INDEX idx_team_members_user_id ON team_members(user_id);
CREATE INDEX idx_team_members_department ON team_members(department);
CREATE INDEX idx_team_members_visibility ON team_members(visibility);
CREATE INDEX idx_team_members_is_active ON team_members(is_active);
