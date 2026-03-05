"""CLI entrypoint for RustDesk Server Friendly helper."""

from __future__ import annotations

import argparse
from pathlib import Path

from .content import SUPPORTED_TARGETS, SUPPORTED_TOPICS, render_guide


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="rustdesk-friendly",
        description=(
            "Friendly RustDesk self-host helper for deploy, service setup, log limits, "
            "and migration guidance"
        ),
    )

    sub = parser.add_subparsers(dest="command")

    guide = sub.add_parser("guide", help="Generate markdown guide text")
    guide.add_argument("--target", choices=SUPPORTED_TARGETS, default="linux")
    guide.add_argument("--topic", choices=SUPPORTED_TOPICS, default="all")
    guide.add_argument("--host", default="<PUBLIC_HOST_OR_IP>")
    guide.add_argument("--windows-dir", default=r"C:\RustDesk-Server")
    guide.add_argument("--linux-install-dir", default="/opt/rustdesk-server")
    guide.add_argument("--linux-data-dir", default="/var/lib/rustdesk-server")
    guide.add_argument("--linux-log-dir", default="/var/log/rustdesk-server")
    guide.add_argument("--migration-source", choices=("linux", "windows"), default="windows")
    guide.add_argument("--migration-target", choices=("linux", "windows"), default="linux")
    guide.add_argument("--source-windows-dir", default=r"C:\RustDesk-Server")
    guide.add_argument("--target-windows-dir", default=r"C:\RustDesk-Server")
    guide.add_argument("--source-linux-data-dir", default="/var/lib/rustdesk-server")
    guide.add_argument("--target-linux-data-dir", default="/var/lib/rustdesk-server")
    guide.add_argument("--output", type=Path, help="Write output markdown to this file")

    wizard = sub.add_parser("wizard", help="Run interactive wizard (recommended)")
    wizard.add_argument("--output", type=Path, help="Auto save generated guide to this file")

    sub.add_parser("gui", help="Launch desktop GUI")

    return parser


def _run_guide(args: argparse.Namespace) -> None:
    content = render_guide(
        target=args.target,
        topic=args.topic,
        host=args.host,
        windows_dir=args.windows_dir,
        linux_install_dir=args.linux_install_dir,
        linux_data_dir=args.linux_data_dir,
        linux_log_dir=args.linux_log_dir,
        migration_source_os=args.migration_source,
        migration_target_os=args.migration_target,
        migration_source_windows_dir=args.source_windows_dir,
        migration_target_windows_dir=args.target_windows_dir,
        migration_source_linux_data_dir=args.source_linux_data_dir,
        migration_target_linux_data_dir=args.target_linux_data_dir,
    )
    if args.output:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(content, encoding="utf-8")
    else:
        print(content)


def _run_wizard(output: Path | None) -> None:
    from .wizard import run_wizard

    run_wizard(default_output=output)


def main() -> None:
    parser = build_parser()
    args = parser.parse_args()

    if args.command == "gui":
        try:
            from .gui import launch_gui
        except ModuleNotFoundError as exc:
            raise SystemExit(
                "GUI dependencies are unavailable in this Python build. "
                "Install a Python distribution with tkinter support."
            ) from exc
        launch_gui()
        return

    if args.command == "guide":
        _run_guide(args)
        return

    if args.command == "wizard":
        _run_wizard(args.output)
        return

    # Default no-arg behavior: start interactive wizard
    _run_wizard(output=None)


if __name__ == "__main__":
    main()
