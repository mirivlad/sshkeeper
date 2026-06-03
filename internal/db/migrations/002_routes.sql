-- v0.2.0: Add route support
-- Adds route columns to servers table and migrates existing ProxyJump data.

ALTER TABLE servers ADD COLUMN route_hops TEXT NOT NULL DEFAULT '';

-- Migrate existing ProxyJump values into Route.Hops (all as raw addresses).
-- ProxyJump format: "host1,host2,host3" → each becomes a RouteHop with IsProfile=false.
-- We store as a simple comma-separated list of raw addresses in route_hops for now.
-- The application layer will parse this into Route.Hops on read.
UPDATE servers SET route_hops = proxy_jump WHERE proxy_jump != '' AND route_hops = '';
