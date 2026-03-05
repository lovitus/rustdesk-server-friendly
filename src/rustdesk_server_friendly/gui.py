"""Tkinter GUI for RustDesk server friendly helper."""

from __future__ import annotations

import tkinter as tk
from pathlib import Path
from tkinter import filedialog, messagebox, ttk

from .content import SUPPORTED_TARGETS, SUPPORTED_TOPICS, render_guide


class FriendlyGui:
    def __init__(self, root: tk.Tk) -> None:
        self.root = root
        self.root.title("RustDesk Server Friendly")
        self.root.geometry("1080x760")

        self.target_var = tk.StringVar(value="linux")
        self.topic_var = tk.StringVar(value="all")
        self.host_var = tk.StringVar(value="<PUBLIC_HOST_OR_IP>")
        self.windows_dir_var = tk.StringVar(value=r"C:\RustDesk-Server")
        self.linux_install_var = tk.StringVar(value="/opt/rustdesk-server")
        self.linux_data_var = tk.StringVar(value="/var/lib/rustdesk-server")
        self.linux_log_var = tk.StringVar(value="/var/log/rustdesk-server")

        self._build_ui()
        self.generate()

    def _build_ui(self) -> None:
        top = ttk.Frame(self.root, padding=10)
        top.pack(fill=tk.X)

        ttk.Label(top, text="Target").grid(row=0, column=0, sticky="w", padx=4, pady=4)
        target = ttk.Combobox(top, textvariable=self.target_var, values=SUPPORTED_TARGETS, state="readonly", width=12)
        target.grid(row=0, column=1, sticky="w", padx=4, pady=4)
        target.bind("<<ComboboxSelected>>", self._on_target_change)

        ttk.Label(top, text="Topic").grid(row=0, column=2, sticky="w", padx=4, pady=4)
        topic = ttk.Combobox(top, textvariable=self.topic_var, values=SUPPORTED_TOPICS, state="readonly", width=12)
        topic.grid(row=0, column=3, sticky="w", padx=4, pady=4)

        ttk.Label(top, text="Public Host").grid(row=0, column=4, sticky="w", padx=4, pady=4)
        ttk.Entry(top, textvariable=self.host_var, width=26).grid(row=0, column=5, sticky="we", padx=4, pady=4)

        ttk.Label(top, text="Windows Root").grid(row=1, column=0, sticky="w", padx=4, pady=4)
        ttk.Entry(top, textvariable=self.windows_dir_var, width=30).grid(row=1, column=1, columnspan=2, sticky="we", padx=4, pady=4)

        ttk.Label(top, text="Linux Install").grid(row=1, column=3, sticky="w", padx=4, pady=4)
        ttk.Entry(top, textvariable=self.linux_install_var, width=30).grid(row=1, column=4, columnspan=2, sticky="we", padx=4, pady=4)

        ttk.Label(top, text="Linux Data").grid(row=2, column=0, sticky="w", padx=4, pady=4)
        ttk.Entry(top, textvariable=self.linux_data_var, width=30).grid(row=2, column=1, columnspan=2, sticky="we", padx=4, pady=4)

        ttk.Label(top, text="Linux Log").grid(row=2, column=3, sticky="w", padx=4, pady=4)
        ttk.Entry(top, textvariable=self.linux_log_var, width=30).grid(row=2, column=4, columnspan=2, sticky="we", padx=4, pady=4)

        btns = ttk.Frame(self.root, padding=(10, 0, 10, 10))
        btns.pack(fill=tk.X)

        ttk.Button(btns, text="Generate", command=self.generate).pack(side=tk.LEFT, padx=4)
        ttk.Button(btns, text="Copy", command=self.copy_to_clipboard).pack(side=tk.LEFT, padx=4)
        ttk.Button(btns, text="Save As", command=self.save_to_file).pack(side=tk.LEFT, padx=4)

        self.status_label = ttk.Label(btns, text="Ready")
        self.status_label.pack(side=tk.RIGHT, padx=4)

        body = ttk.Frame(self.root, padding=(10, 0, 10, 10))
        body.pack(fill=tk.BOTH, expand=True)

        self.output = tk.Text(body, wrap="word", font=("Menlo", 11))
        self.output.pack(side=tk.LEFT, fill=tk.BOTH, expand=True)

        scrollbar = ttk.Scrollbar(body, orient=tk.VERTICAL, command=self.output.yview)
        scrollbar.pack(side=tk.RIGHT, fill=tk.Y)
        self.output.configure(yscrollcommand=scrollbar.set)

    def _on_target_change(self, _event: object = None) -> None:
        target = self.target_var.get()
        if target == "cross":
            self.topic_var.set("migrate")
        self.generate()

    def generate(self) -> None:
        try:
            text = render_guide(
                target=self.target_var.get(),
                topic=self.topic_var.get(),
                host=self.host_var.get(),
                windows_dir=self.windows_dir_var.get(),
                linux_install_dir=self.linux_install_var.get(),
                linux_data_dir=self.linux_data_var.get(),
                linux_log_dir=self.linux_log_var.get(),
            )
        except Exception as exc:  # pragma: no cover - GUI path
            messagebox.showerror("Generate failed", str(exc))
            self.status_label.config(text="Generate failed")
            return

        self.output.delete("1.0", tk.END)
        self.output.insert("1.0", text)
        self.status_label.config(text="Guide generated")

    def copy_to_clipboard(self) -> None:
        text = self.output.get("1.0", tk.END).strip()
        self.root.clipboard_clear()
        self.root.clipboard_append(text)
        self.status_label.config(text="Copied to clipboard")

    def save_to_file(self) -> None:
        filename = filedialog.asksaveasfilename(
            title="Save guide",
            defaultextension=".md",
            filetypes=[("Markdown", "*.md"), ("Text", "*.txt"), ("All files", "*.*")],
        )
        if not filename:
            return
        Path(filename).write_text(self.output.get("1.0", tk.END), encoding="utf-8")
        self.status_label.config(text=f"Saved: {filename}")


def launch_gui() -> None:
    root = tk.Tk()
    FriendlyGui(root)
    root.mainloop()
