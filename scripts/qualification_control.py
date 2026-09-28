"""Short-lived SSH control channel; EOF stops future work, never replays writes."""
from contextlib import contextmanager
import json
import os
from pathlib import Path
import select
import signal
import subprocess
import tempfile
import threading
import time


def cancelled(signum, frame):
    raise InterruptedError('Qualification control was interrupted')


@contextmanager
def cancellable():
    if threading.current_thread() is not threading.main_thread():
        # The process's main supervisor owns signals; pipe EOF still cancels peers.
        yield
        return
    previous = {sig: signal.getsignal(sig) for sig in (signal.SIGTERM, signal.SIGHUP)}
    for sig in previous:
        signal.signal(sig, cancelled)
    try:
        yield
    finally:
        for sig, handler in previous.items():
            signal.signal(sig, handler)


def descendants(pid):
    """Capture owned Linux descendants before any parent is terminated."""
    processes = {}
    for path in Path('/proc').glob('[0-9]*'):
        try:
            fields = (path / 'stat').read_text().rsplit(')', 1)[1].split()
            processes[int(path.name)] = (int(fields[1]), fields[19])
        except (OSError, ValueError, IndexError):
            continue
    owned = {pid}
    while True:
        children = {child for child, (parent, _) in processes.items() if parent in owned}
        if children <= owned:
            return {child: processes[child][1] for child in owned if child in processes}
        owned.update(children)


def stop_child(process):
    owned = descendants(process.pid)
    def send(sig):
        # A process group also catches children created after the snapshot.
        try:
            os.killpg(process.pid, sig)
        except ProcessLookupError:
            pass
        for pid, birth in owned.items():
            try:
                actual = (Path('/proc') / str(pid) / 'stat').read_text().rsplit(')', 1)[1].split()[19]
                if actual == birth:
                    os.kill(pid, sig)
            except (OSError, IndexError):
                pass
    send(signal.SIGTERM)
    try:
        process.wait(timeout=10)
    except subprocess.TimeoutExpired:
        pass
    send(signal.SIGKILL)
    process.wait()


def run_child(argv, payload, stdout, stderr, timeout, **kwargs):
    with subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=stdout, stderr=stderr,
                          start_new_session=True, **kwargs) as process:
        try:
            process.communicate(payload, timeout=timeout)
        except BaseException:
            stop_child(process)
            raise
        if process.returncode:
            raise ValueError('Qualification child failed; retain private receipts')


class Lifeline:
    def __init__(self, stream, timeout=30):
        self.fd, self.timeout = stream.fileno(), timeout
        self.buffer = b''
        self.stopped = threading.Event()

    def line(self, timeout, limit):
        deadline = time.monotonic() + timeout
        while not self.stopped.is_set():
            if b'\n' in self.buffer:
                line, self.buffer = self.buffer.split(b'\n', 1)
                if len(line) > limit:
                    raise ValueError('Oversized control frame')
                return line
            if len(self.buffer) > limit or time.monotonic() >= deadline:
                raise ValueError('Control channel expired')
            if select.select([self.fd], [], [], min(0.2, max(0, deadline-time.monotonic())))[0]:
                chunk = os.read(self.fd, 65536)
                if not chunk:
                    raise ValueError('Control channel closed')
                self.buffer += chunk
        return None

    def watch(self):
        try:
            while not self.stopped.is_set():
                frame = self.line(self.timeout, 16)
                if frame is not None and frame != b'ping':
                    raise ValueError('Invalid control heartbeat')
        except Exception:
            if not self.stopped.is_set():
                os.kill(os.getpid(), signal.SIGTERM)

    def start(self):
        self.thread = threading.Thread(target=self.watch, daemon=True)
        self.thread.start()

    def close(self):
        self.stopped.set()
        self.thread.join(timeout=1)


def serve(callback, stream, timeout=30):
    """Read one JSON line, then require heartbeats until the callback finishes."""
    with cancellable():
        life = Lifeline(stream, timeout)
        request = json.loads(life.line(60, 4*1024*1024))
        life.start()
        try:
            return callback(request)
        finally:
            life.close()


def transport(argv, request, timeout=86400):
    """Keep the same SSH stdin open; caller death/connection loss is remote EOF."""
    payload = json.dumps(request, sort_keys=True, separators=(',', ':')).encode()+b'\n'
    with cancellable(), tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as errors:
        with subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=output, stderr=errors,
                              start_new_session=True) as process:
            try:
                process.stdin.write(payload)
                process.stdin.flush()
                deadline = time.monotonic()+timeout
                while True:
                    try:
                        process.wait(timeout=min(5, max(0, deadline-time.monotonic())))
                        break
                    except subprocess.TimeoutExpired:
                        if time.monotonic() >= deadline:
                            raise TimeoutError('Qualification transport expired')
                        process.stdin.write(b'ping\n')
                        process.stdin.flush()
                if process.returncode:
                    raise ValueError('Qualification transport failed; retain remote intents')
            except BaseException:
                try:
                    process.stdin.close()  # Remote EOF stops any later stage.
                except OSError:
                    pass
                stop_child(process)
                raise
            finally:
                try:
                    process.stdin.close()
                except OSError:
                    pass
        output.seek(0)
        result = output.read(8*1024*1024+1)
        if len(result) > 8*1024*1024:
            raise ValueError('Oversized qualification result')
        return result.decode()


def worker(argv, request):
    """Wrap a nested worker in the same finite channel and owned-child cleanup."""
    with tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as errors:
        run_child(argv, json.dumps(request).encode(), output, errors, 14400)
        output.seek(0)
        raw = output.read(8*1024*1024+1)
        if len(raw) > 8*1024*1024:
            raise ValueError('Oversized remote worker result')
        return json.loads(raw)


def inline_receiver(source, expected_hash, argv):
    """Verified control bytes travel with the existing SSH command, without keys."""
    import base64
    import hashlib
    if hashlib.sha256(source).hexdigest() != expected_hash:
        raise ValueError('Control helper bytes changed')
    code = ("import base64,hashlib,os,pathlib,sys; b=base64.b64decode(sys.argv[1]); "
            "assert hashlib.sha256(b).hexdigest()==sys.argv[2]; os.chdir(pathlib.Path.home()); "
            "sys.argv=['qualification_control.py','--receive']+sys.argv[3:]; "
            "exec(compile(b,'qualification_control.py','exec'),{'__name__':'__main__','__file__':'qualification_control.py'})")
    return ['python3', '-B', '-c', code, base64.b64encode(source).decode(), expected_hash, *argv]


if __name__ == '__main__':
    import sys
    if sys.argv[1:2] != ['--receive'] or len(sys.argv) < 3:
        raise SystemExit('A supervised worker command is required')
    try:
        print(json.dumps(serve(lambda request: worker(sys.argv[2:], request), sys.stdin)))
    except Exception:
        raise SystemExit('Remote control ended; retain unknown intents and owned resources. No replay.') from None
