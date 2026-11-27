"""to-do list for the Kindle dashboard, kept in a plain text file on the PC.

One task per line. A line starting with "x " is done (the todo.txt convention).
Blank lines and lines starting with "#" are ignored. Tapping a task on the
Kindle adds or removes the "x " and saves the file.
"""
import os
import tempfile
import threading
from pathlib import Path

KINDLE_ROWS = 8
MAX_TEXT = 120

TEMPLATE = """# Kindle Control Deck to-do list
# One task per line. Start a line with "x " to mark it done.
# Tap a task on the Kindle to tick it off. Lines starting with # are ignored.
Add your first task here
x This task is already done
"""


class TodoUnavailable(Exception):
    pass


def _parse(line):
    """Return (text, done) for a task line, or None for blank and comment lines."""
    stripped = line.strip()
    if not stripped or stripped.startswith("#"):
        return None
    if stripped.startswith("x ") or stripped == "x":
        return stripped[2:].strip()[:MAX_TEXT], True
    return stripped[:MAX_TEXT], False


class TodoFile:
    def __init__(self, path):
        self.path = Path(path)
        self._lock = threading.Lock()
        if not self.path.exists():
            self.path.write_text(TEMPLATE, encoding="utf-8")

    def _read(self):
        raw = self.path.read_bytes()
        bom = raw.startswith(b"\xef\xbb\xbf")  # Notepad may add a byte order mark.
        return raw.decode("utf-8-sig", errors="replace").splitlines(keepends=True), bom

    def snapshot(self):
        try:
            with self._lock:
                lines, _ = self._read()
        except OSError:
            return {"items": [], "done": 0, "total": 0, "error": "unreadable"}
        tasks = []
        for number, line in enumerate(lines):
            parsed = _parse(line)
            if parsed:
                tasks.append({"line": number, "text": parsed[0], "done": parsed[1]})
        done = sum(task["done"] for task in tasks)
        # Open tasks first so the Kindle's eight rows show what is left to do.
        ordered = [task for task in tasks if not task["done"]] + [task for task in tasks if task["done"]]
        return {"items": ordered[:KINDLE_ROWS], "done": done, "total": len(tasks)}

    def toggle(self, number, text):
        with self._lock:
            try:
                lines, bom = self._read()
            except OSError as exc:
                raise TodoUnavailable("The PC could not open todo.txt.") from exc
            # The file may have been edited since the Kindle last looked; find the task by its text if it moved.
            if not (0 <= number < len(lines) and (_parse(lines[number]) or ("",))[0] == text):
                matches = [index for index, line in enumerate(lines) if (_parse(line) or ("",))[0] == text]
                if not matches:
                    raise TodoUnavailable("That task changed on the PC. The list will refresh in a moment.")
                number = matches[0]
            line = lines[number]
            ending = line[len(line.rstrip("\r\n")):]
            body = line.rstrip("\r\n")
            indent = body[:len(body) - len(body.lstrip())]
            content = body.strip()
            if _parse(line)[1]:
                content = content[1:].lstrip()
            else:
                content = "x " + content
            lines[number] = indent + content + ending
            data = "".join(lines).encode("utf-8")
            if bom:
                data = b"\xef\xbb\xbf" + data
            handle, temp = tempfile.mkstemp(dir=self.path.parent, prefix=".todo-", suffix=".tmp")
            try:
                with os.fdopen(handle, "wb") as file:
                    file.write(data)
                os.replace(temp, self.path)
            except OSError as exc:
                try:
                    os.remove(temp)
                except OSError:
                    pass
                raise TodoUnavailable("The PC could not save todo.txt. Close it in other apps and try again.") from exc
