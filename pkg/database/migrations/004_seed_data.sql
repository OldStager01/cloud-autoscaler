-- 004_seed_data.sql
-- Seed data for development and testing

-- Insert default admin user (password: admin123)
-- Hash generated using bcrypt
INSERT INTO users (username, password_hash) 
VALUES ('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMy.MqrqQlLl1eWHvYPsHy8gkPu9M7Mq8Wy')
ON CONFLICT (username) DO NOTHING;

-- Insert sample clusters
INSERT INTO clusters (id, name, min_servers, max_servers, status, config) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'web-production', 3, 20, 'active', 
     '{"collector_endpoint": "http://localhost:9000/metrics", "target_cpu": 70}'),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'api-production', 2, 15, 'active',
     '{"collector_endpoint":  "http://localhost:9000/metrics", "target_cpu": 75}'),
    ('c0eebc99-9c0b-4ef8-bb6d-6bb9bd380a33', 'worker-batch', 2, 10, 'paused',
     '{"collector_endpoint": "http://localhost:9000/metrics", "target_cpu": 80}')
ON CONFLICT DO NOTHING;

-- Insert sample servers for web-production cluster
INSERT INTO servers (cluster_id, state, activated_at) VALUES
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'ACTIVE', NOW() - INTERVAL '2 days'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'ACTIVE', NOW() - INTERVAL '2 days'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'ACTIVE', NOW() - INTERVAL '1 day'),
    ('a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11', 'PROVISIONING', NULL)
ON CONFLICT DO NOTHING;

-- Insert sample servers for api-production cluster
INSERT INTO servers (cluster_id, state, activated_at) VALUES
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'ACTIVE', NOW() - INTERVAL '3 days'),
    ('b0eebc99-9c0b-4ef8-bb6d-6bb9bd380a22', 'ACTIVE', NOW() - INTERVAL '3 days')
ON CONFLICT DO NOTHING;