from rustdesk_server_friendly.content import render_guide


def test_linux_all_contains_required_sections() -> None:
    output = render_guide(target="linux", topic="all", host="example.com")
    assert "Linux CLI Deploy" in output
    assert "Linux Service Install" in output
    assert "Linux Log Limits" in output
    assert "example.com:21117" in output


def test_windows_all_contains_pm2() -> None:
    output = render_guide(target="windows", topic="all", host="1.2.3.4")
    assert "Windows Service Install (PM2)" in output
    assert "pm2 start" in output


def test_cross_migration_contains_key_files() -> None:
    output = render_guide(target="cross", topic="migrate")
    assert "id_ed25519" in output
    assert "db_v2.sqlite3" in output


def test_invalid_target_raises() -> None:
    try:
        render_guide(target="macos", topic="all")
    except ValueError as exc:
        assert "Unsupported target" in str(exc)
    else:
        raise AssertionError("Expected ValueError")
