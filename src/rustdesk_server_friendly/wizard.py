"""Interactive CLI wizard for RustDesk server runbook generation."""

from __future__ import annotations

from pathlib import Path

from .content import (
    SUPPORTED_MIGRATION_OS,
    SUPPORTED_TARGETS,
    SUPPORTED_TOPICS,
    render_guide,
)


def _prompt_text(label: str, default: str) -> str:
    raw = input(f"{label} [{default}]: ").strip()
    return raw or default


def _prompt_choice(label: str, options: tuple[str, ...], default: str) -> str:
    print(label)
    for idx, item in enumerate(options, start=1):
        mark = " (default)" if item == default else ""
        print(f"  {idx}. {item}{mark}")

    while True:
        raw = input(f"Select 1-{len(options)} or value [{default}]: ").strip().lower()
        if not raw:
            return default
        if raw in options:
            return raw
        if raw.isdigit():
            index = int(raw)
            if 1 <= index <= len(options):
                return options[index - 1]
        print("Invalid selection. Try again.")


def _prompt_yes_no(label: str, default_yes: bool = True) -> bool:
    suffix = "[Y/n]" if default_yes else "[y/N]"
    while True:
        raw = input(f"{label} {suffix}: ").strip().lower()
        if not raw:
            return default_yes
        if raw in ("y", "yes"):
            return True
        if raw in ("n", "no"):
            return False
        print("Please answer y or n.")


def run_wizard(default_output: Path | None = None) -> None:
    print("RustDesk Server Friendly Wizard")
    print("Generate deployment/service/log/migration runbooks with guided prompts.")
    print("")

    mode = _prompt_choice(
        "Choose workflow",
        options=("guided-setup", "guided-migration", "custom"),
        default="guided-setup",
    )

    target = "linux"
    topic = "all"

    host = "<PUBLIC_HOST_OR_IP>"
    windows_dir = r"C:\RustDesk-Server"
    linux_install_dir = "/opt/rustdesk-server"
    linux_data_dir = "/var/lib/rustdesk-server"
    linux_log_dir = "/var/log/rustdesk-server"

    migration_source_os = "windows"
    migration_target_os = "linux"
    migration_source_windows_dir = r"C:\RustDesk-Server"
    migration_target_windows_dir = r"C:\RustDesk-Server"
    migration_source_linux_data_dir = "/var/lib/rustdesk-server"
    migration_target_linux_data_dir = "/var/lib/rustdesk-server"

    if mode == "guided-setup":
        target = _prompt_choice("Target OS", options=("linux", "windows"), default="linux")
        topic = "all"
        host = _prompt_text("Public host/IP used by hbbs -r", host)

        if not _prompt_yes_no(
            "Auto-detect runtime paths from running services/processes (recommended)?",
            default_yes=True,
        ):
            if target == "linux":
                linux_install_dir = _prompt_text("Linux install dir (fallback)", linux_install_dir)
                linux_data_dir = _prompt_text("Linux data dir (fallback)", linux_data_dir)
                linux_log_dir = _prompt_text("Linux log dir (fallback)", linux_log_dir)
            else:
                windows_dir = _prompt_text("Windows root dir (fallback)", windows_dir)

    elif mode == "guided-migration":
        target = "cross"
        topic = "migrate"
        migration_source_os = _prompt_choice("Migration source OS", SUPPORTED_MIGRATION_OS, "windows")
        migration_target_os = _prompt_choice("Migration target OS", SUPPORTED_MIGRATION_OS, "linux")

        if not _prompt_yes_no(
            "Auto-detect source/target data paths from running services (recommended)?",
            default_yes=True,
        ):
            if migration_source_os == "linux":
                migration_source_linux_data_dir = _prompt_text(
                    "Source Linux data dir (fallback)", migration_source_linux_data_dir
                )
            else:
                migration_source_windows_dir = _prompt_text(
                    "Source Windows root dir (fallback)", migration_source_windows_dir
                )

            if migration_target_os == "linux":
                migration_target_linux_data_dir = _prompt_text(
                    "Target Linux data dir (fallback)", migration_target_linux_data_dir
                )
            else:
                migration_target_windows_dir = _prompt_text(
                    "Target Windows root dir (fallback)", migration_target_windows_dir
                )

    else:
        target = _prompt_choice("Target", SUPPORTED_TARGETS, "linux")
        topic = _prompt_choice("Topic", SUPPORTED_TOPICS, "all")

        if topic == "migrate":
            migration_source_os = _prompt_choice("Migration source OS", SUPPORTED_MIGRATION_OS, "windows")
            migration_target_os = _prompt_choice("Migration target OS", SUPPORTED_MIGRATION_OS, "linux")
            if not _prompt_yes_no(
                "Auto-detect source/target data paths from running services (recommended)?",
                default_yes=True,
            ):
                if migration_source_os == "linux":
                    migration_source_linux_data_dir = _prompt_text(
                        "Source Linux data dir (fallback)", migration_source_linux_data_dir
                    )
                else:
                    migration_source_windows_dir = _prompt_text(
                        "Source Windows root dir (fallback)", migration_source_windows_dir
                    )

                if migration_target_os == "linux":
                    migration_target_linux_data_dir = _prompt_text(
                        "Target Linux data dir (fallback)", migration_target_linux_data_dir
                    )
                else:
                    migration_target_windows_dir = _prompt_text(
                        "Target Windows root dir (fallback)", migration_target_windows_dir
                    )
        else:
            host = _prompt_text("Public host/IP used by hbbs -r", host)
            if not _prompt_yes_no(
                "Auto-detect runtime paths from running services/processes (recommended)?",
                default_yes=True,
            ):
                if target == "linux":
                    linux_install_dir = _prompt_text("Linux install dir (fallback)", linux_install_dir)
                    linux_data_dir = _prompt_text("Linux data dir (fallback)", linux_data_dir)
                    linux_log_dir = _prompt_text("Linux log dir (fallback)", linux_log_dir)
                elif target == "windows":
                    windows_dir = _prompt_text("Windows root dir (fallback)", windows_dir)

    guide = render_guide(
        target=target,
        topic=topic,
        host=host,
        windows_dir=windows_dir,
        linux_install_dir=linux_install_dir,
        linux_data_dir=linux_data_dir,
        linux_log_dir=linux_log_dir,
        migration_source_os=migration_source_os,
        migration_target_os=migration_target_os,
        migration_source_windows_dir=migration_source_windows_dir,
        migration_target_windows_dir=migration_target_windows_dir,
        migration_source_linux_data_dir=migration_source_linux_data_dir,
        migration_target_linux_data_dir=migration_target_linux_data_dir,
    )

    print("\n===== Generated Guide =====\n")
    print(guide)

    output_path: Path | None = default_output
    if output_path is None and _prompt_yes_no("Export guide to file?", default_yes=True):
        default_name = (
            f"rustdesk-{migration_source_os}-to-{migration_target_os}-migration.md"
            if topic == "migrate"
            else f"rustdesk-{target}-{topic}-guide.md"
        )
        output_path = Path(_prompt_text("Output file", default_name))

    if output_path is not None:
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(guide, encoding="utf-8")
        print(f"Saved: {output_path}")
