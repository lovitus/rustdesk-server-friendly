from rustdesk_server_friendly.content import render_guide


def test_linux_all_contains_required_sections_and_idempotent_markers() -> None:
    output = render_guide(target="linux", topic="all", host="example.com")
    assert "Linux CLI Deploy (Binary, Idempotent)" in output
    assert "Linux Service Install (systemd, Idempotent)" in output
    assert "Linux Log Limits (Idempotent)" in output
    assert "example.com:21117" in output
    assert "[SKIP]" in output
    assert "[STOP]" in output


def test_windows_all_contains_pm2_and_idempotent_markers() -> None:
    output = render_guide(target="windows", topic="all", host="1.2.3.4")
    assert "Windows Service Install (PM2, Idempotent)" in output
    assert "pm2 start" in output
    assert "[SKIP]" in output


def test_cross_migration_windows_to_linux_contains_key_files() -> None:
    output = render_guide(
        target="cross",
        topic="migrate",
        migration_source_os="windows",
        migration_target_os="linux",
    )
    assert "Migration: Windows -> Linux" in output
    assert "id_ed25519" in output
    assert "db_v2.sqlite3" in output
    assert "Resolve-RustDeskDataDir" in output
    assert "resolve_rustdesk_data_dir" in output
    assert "RUSTDESK_SOURCE_DATA_DIR" in output
    assert "RUSTDESK_TARGET_DATA_DIR" in output


def test_cross_migration_linux_to_linux_supported() -> None:
    output = render_guide(
        target="cross",
        topic="migrate",
        migration_source_os="linux",
        migration_target_os="linux",
    )
    assert "Migration: Linux -> Linux" in output
    assert "resolve_target_data_dir" in output
    assert "RUSTDESK_SOURCE_DATA_DIR" in output
    assert "RUSTDESK_TARGET_DATA_DIR" in output


def test_cross_migration_windows_to_windows_supported() -> None:
    output = render_guide(
        target="cross",
        topic="migrate",
        migration_source_os="windows",
        migration_target_os="windows",
    )
    assert "Migration: Windows -> Windows" in output
    assert "Resolve-RustDeskDataDir" in output
    assert "RUSTDESK_SOURCE_DATA_DIR" in output
    assert "RUSTDESK_TARGET_DATA_DIR" in output


def test_cross_migration_linux_to_windows_supported() -> None:
    output = render_guide(
        target="cross",
        topic="migrate",
        migration_source_os="linux",
        migration_target_os="windows",
    )
    assert "Migration: Linux -> Windows" in output


def test_invalid_target_raises() -> None:
    try:
        render_guide(target="macos", topic="all")
    except ValueError as exc:
        assert "Unsupported target" in str(exc)
    else:
        raise AssertionError("Expected ValueError")


def test_invalid_migration_source_raises() -> None:
    try:
        render_guide(target="cross", topic="migrate", migration_source_os="solaris")
    except ValueError as exc:
        assert "Unsupported migration source OS" in str(exc)
    else:
        raise AssertionError("Expected ValueError")
