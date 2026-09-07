-- Demo v0.3.2: Runtime Host onboarding metadata.  The existing `host_id`
-- column on runtimes remains the compatible database representation of the
-- public RuntimeHost relationship (runtime_host_id in API/UI terminology).

ALTER TABLE runtime_hosts
  ADD COLUMN ssh_username VARCHAR(160) NOT NULL DEFAULT '' AFTER ssh_port,
  ADD COLUMN docker_socket_path VARCHAR(255) NOT NULL DEFAULT '/var/run/docker.sock' AFTER docker_endpoint,
  ADD COLUMN docker_binary VARCHAR(160) NOT NULL DEFAULT 'docker' AFTER docker_socket_path,
  ADD COLUMN description TEXT NULL AFTER labels,
  ADD COLUMN cpu_actual VARCHAR(40) NOT NULL DEFAULT '0 CPU' AFTER cpu_allocated,
  ADD COLUMN memory_actual VARCHAR(40) NOT NULL DEFAULT '0 GB' AFTER memory_allocated,
  ADD COLUMN storage_actual VARCHAR(40) NOT NULL DEFAULT '0 GB' AFTER storage_allocated,
  ADD COLUMN container_count INT NOT NULL DEFAULT 0 AFTER runtime_count;

-- `healthy` was the v0.3 mock-provider status. v0.3.2 uses infrastructure
-- states consistently while preserving existing host records.
UPDATE runtime_hosts SET status='online' WHERE status='healthy';
