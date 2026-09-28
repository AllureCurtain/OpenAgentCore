"""Real child/process control fixtures; no deployment, provider or model calls."""
import hashlib
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import time
import unittest
from unittest import mock
from concurrent.futures import ThreadPoolExecutor
import qualification_control as control


def alive(pid):
    try:
        return (Path('/proc')/str(pid)/'stat').read_text().rsplit(')',1)[1].split()[0] != 'Z'
    except FileNotFoundError:
        return False


class ControlTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.root=Path(self.temp.name)
        self.source=Path(control.__file__).read_bytes()
        self.digest=hashlib.sha256(self.source).hexdigest()

    def receiver(self,worker):
        return control.inline_receiver(self.source,self.digest,[sys.executable,'-B','-c',worker])

    def await_file(self,path):
        deadline=time.monotonic()+5
        while not path.exists() and time.monotonic()<deadline:time.sleep(.02)
        self.assertTrue(path.exists(),'Control child never reached ready point')
        return json.loads(path.read_text())

    def sleeping_worker(self):
        ready=self.root/'ready.json';late=self.root/'late'
        grandchild="import time;from pathlib import Path;time.sleep(2);Path("+repr(str(late))+").write_text('escaped');time.sleep(60)"
        code="import json,os,subprocess,sys,time;from pathlib import Path; child=subprocess.Popen([sys.executable,'-c',"+repr(grandchild)+"]);Path("+repr(str(ready))+").write_text(json.dumps({'worker':os.getpid(),'grandchild':child.pid}));time.sleep(60)"
        return code,ready,late

    def assert_stopped(self,pids):
        deadline=time.monotonic()+3
        while any(alive(pid) for pid in pids.values()) and time.monotonic()<deadline:time.sleep(.02)
        self.assertFalse(any(alive(pid) for pid in pids.values()))

    def test_normal_completion_with_heartbeat_and_nested_worker(self):
        worker="import json,sys,time;q=json.load(sys.stdin);time.sleep(5.2);print(json.dumps({'observed':q['value']}))"
        result=control.transport(self.receiver(worker),{'value':'fixture'},timeout=10)
        self.assertEqual(json.loads(result),{'observed':'fixture'})

    def test_eof_and_real_signals_stop_foreground_descendants(self):
        for reason in ('eof',signal.SIGTERM,signal.SIGHUP):
            with self.subTest(reason=reason):
                code,ready,late=self.sleeping_worker()
                with tempfile.TemporaryFile() as out,tempfile.TemporaryFile() as err:
                    process=subprocess.Popen(self.receiver(code),stdin=subprocess.PIPE,stdout=out,stderr=err,start_new_session=True)
                    try:
                        process.stdin.write(b'{}\n');process.stdin.flush();pids=self.await_file(ready)
                        if reason=='eof':process.stdin.close()
                        else:process.send_signal(reason)
                        process.wait(timeout=5)
                        self.assertNotEqual(process.returncode,0)
                        self.assert_stopped(pids)
                        self.assertFalse(late.exists())
                    finally:
                        if process.poll() is None:control.stop_child(process)
                        if not process.stdin.closed:process.stdin.close()
                        ready.unlink(missing_ok=True)

    def test_silent_open_channel_expires_and_stops_work(self):
        worker,ready,_=self.sleeping_worker()
        receiver="import sys,json;sys.path.insert(0,"+repr(str(Path(control.__file__).parent))+");import qualification_control as c; print(json.dumps(c.serve(lambda q:c.worker([sys.executable,'-c',"+repr(worker)+"],q),sys.stdin,timeout=.3)))"
        with tempfile.TemporaryFile() as out,tempfile.TemporaryFile() as err:
            process=subprocess.Popen([sys.executable,'-B','-c',receiver],stdin=subprocess.PIPE,stdout=out,stderr=err,start_new_session=True)
            try:
                process.stdin.write(b'{}\n');process.stdin.flush();pids=self.await_file(ready)
                process.wait(timeout=5);self.assertNotEqual(process.returncode,0);self.assert_stopped(pids)
            finally:
                if process.poll() is None:control.stop_child(process)
                process.stdin.close()

    def test_sender_death_is_receiver_eof(self):
        worker,ready,_=self.sleeping_worker();argv=self.receiver(worker)
        sender="import sys;sys.path.insert(0,"+repr(str(Path(control.__file__).parent))+");import qualification_control as c;c.transport("+repr(argv)+",{},timeout=60)"
        with tempfile.TemporaryFile() as out,tempfile.TemporaryFile() as err:
            process=subprocess.Popen([sys.executable,'-B','-c',sender],stdout=out,stderr=err,start_new_session=True)
            try:
                pids=self.await_file(ready);process.kill();process.wait(timeout=5);self.assert_stopped(pids)
            finally:
                if process.poll() is None:control.stop_child(process)

    def test_parallel_worker_transports_do_not_install_thread_signal_handlers(self):
        receiver=self.receiver("import json,sys;print(json.dumps(json.load(sys.stdin)))")
        with ThreadPoolExecutor(max_workers=2) as pool:
            results=list(pool.map(lambda value:json.loads(control.transport(receiver,{'value':value},timeout=5)),(1,2)))
        self.assertEqual(results,[{'value':1},{'value':2}])

    def test_three_level_exit_paths_reap_foreground_and_preserve_background(self):
        for reason in ('timeout', 'kill', 'nonzero', 'normal'):
            with self.subTest(reason=reason):
                ready = self.root / (reason + '-foreground.json')
                background = self.root / (reason + '-background.json')
                foreground = ("import os,json,time;from pathlib import Path;Path(" + repr(str(ready))
                    + ").write_text(json.dumps({'foreground':os.getpid()}));time.sleep(60)")
                middle = ("import subprocess,sys,time; p=subprocess.Popen([sys.executable,'-c',"
                    + repr(foreground) + "]);time.sleep(60)")
                # This worker stands in for node_worker -> lifecycle -> installer.
                # The outside control owner survives the inner timeout/SIGKILL.
                worker = ("import subprocess,sys,time,json;from pathlib import Path;"
                    "bg=subprocess.Popen([sys.executable,'-c','import time;time.sleep(60)'],start_new_session=True);"
                    "Path(" + repr(str(background)) + ").write_text(json.dumps({'background':bg.pid}));")
                if reason == 'timeout':
                    worker += "subprocess.run([sys.executable,'-c'," + repr(middle) + "],timeout=.3)"
                else:
                    worker += ("p=subprocess.Popen([sys.executable,'-c'," + repr(middle) + "]);"
                        "time.sleep(.3);")
                    if reason == 'kill':
                        worker += "p.kill();p.wait();sys.exit(1)"
                    elif reason == 'nonzero':
                        worker += "sys.exit(7)"
                    else:
                        worker += "print(json.dumps({'completed':True}))"
                bg = None
                try:
                    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
                        if reason == 'normal':
                            control.run_child([sys.executable,'-c',worker], b'', out, err, 3)
                        else:
                            with self.assertRaises(ValueError):
                                control.run_child([sys.executable,'-c',worker], b'', out, err, 3)
                    fg = self.await_file(ready)
                    bg = self.await_file(background)['background']
                    self.assert_stopped(fg)
                    self.assertTrue(alive(bg), 'Explicit background resource was removed')
                finally:
                    if bg is None and background.exists():
                        bg = json.loads(background.read_text())['background']
                    if bg is not None:
                        try: os.kill(bg, signal.SIGKILL)
                        except ProcessLookupError: pass

    def test_sender_child_cleanup_without_linux_proc(self):
        process = subprocess.Popen([sys.executable, '-c', 'import time;time.sleep(60)'])
        try:
            with mock.patch.object(control, 'descendants', return_value={}):
                control.stop_child(process, own_group=False)
            self.assertIsNotNone(process.returncode)
        finally:
            if process.poll() is None:
                process.kill(); process.wait()

    def test_control_bytes_are_pinned(self):
        with self.assertRaises(ValueError):control.inline_receiver(self.source,'0'*64,['python3'])


if __name__=='__main__':unittest.main()
