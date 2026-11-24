# Phase 5: sync, recycle bin and optional system integration

Ant keeps its existing transactional recycle-bin implementation:

- delete = soft delete;
- restore returns the record to the active catalog;
- permanent delete removes managed data through the existing deletion transaction;
- expired trash keeps the existing 72-hour retention cleanup.

HTTP/MCP/frontend expose that lifecycle without returning proxy credentials or
raw launch arguments.

"Sync" is intentionally reported as a backup-channel capability. Ant's existing
backup import/export, conflict preservation and scheduled remote backup remain
the implementation. The status contract explicitly reports that this is not a
continuous bidirectional profile-sync engine.

Optional system integration is opt-in only. On Windows the user may explicitly
register or unregister the per-user `antbrowser://` URL protocol. Ant does not
set itself as the OS default browser, does not modify proxy/TUN/firewall
settings, and performs no system integration automatically during startup.
Non-Windows builds report the protocol action as unsupported.
