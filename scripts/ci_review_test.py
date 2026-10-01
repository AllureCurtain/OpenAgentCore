"""CI notification failure and payload contracts, without external calls."""
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch, MagicMock

spec = importlib.util.spec_from_file_location(
    'ci_review', Path(__file__).resolve().parents[1] / '.github/scripts/ci_review.py')
review = importlib.util.module_from_spec(spec)
spec.loader.exec_module(review)


class NotificationTest(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.env = patch.dict(os.environ, {
            'GITHUB_STEP_SUMMARY': str(Path(self.directory.name) / 'summary'),
            'FEISHU_WEBHOOK_URL': 'https://open.feishu.cn/open-apis/bot/v2/hook/test',
            'FEISHU_WEBHOOK_SECRET': 'test-secret',
            'REVIEW_OUTCOME': 'success',
            'REVIEW_OUTPUT': json.dumps({'summary': '审核结果'}),
        })
        self.env.start()
        self.addCleanup(self.env.stop)
        self.run = dict(name='core-check', conclusion='failure', head_branch='feature',
                        head_sha='a' * 40, run_attempt=2, html_url='https://github.com/run/1')

    def deliver(self, response=None):
        result = MagicMock()
        result.__enter__.return_value.read.return_value = json.dumps(response or {'code': 0})
        with patch.object(review.urllib.request, 'urlopen', return_value=result) as send:
            review.notify(self.run)
        self.assertEqual(send.call_count, 1)
        return json.loads(send.call_args.args[0].data)

    def test_prompt_limit_counts_utf8_bytes(self):
        result = review.bounded("测" * 48000)
        self.assertLess(len(result.encode()), 48100)
        self.assertIn("TRUNCATED", result)

    def test_summary_and_signature(self):
        with patch.object(review.time, 'time', return_value=1700000000):
            payload = self.deliver()
        self.assertEqual(payload['timestamp'], '1700000000')
        expected = review.base64.b64encode(review.hmac.new(
            b'1700000000\ntest-secret', b'', review.hashlib.sha256).digest()).decode()
        self.assertEqual(payload['sign'], expected)
        self.assertIn('审核结果', json.dumps(payload, ensure_ascii=False))
        self.assertIn('failure', json.dumps(payload, ensure_ascii=False))
        self.assertNotIn('test-secret', json.dumps(payload, ensure_ascii=False))

    def test_fixed_card_keeps_model_content_plain(self):
        card = review.build_card(self.run, '<at id=all>all</at>')
        self.assertEqual(card['schema'], '2.0')
        self.assertEqual(card['body']['elements'][2]['text']['tag'], 'plain_text')
        self.assertEqual(card['body']['elements'][-1]['behaviors'][0]['default_url'], self.run['html_url'])

    def test_model_failure_still_notifies(self):
        os.environ['REVIEW_OUTCOME'] = 'failure'
        self.assertIn('审核未完成', json.dumps(self.deliver(), ensure_ascii=False))

    def test_invalid_output_still_notifies(self):
        os.environ['REVIEW_OUTPUT'] = 'invalid'
        self.assertIn('审核未完成', json.dumps(self.deliver(), ensure_ascii=False))

    def test_rejection_fails(self):
        with self.assertRaisesRegex(RuntimeError, 'notification failed'):
            self.deliver({'code': 19021})

    def test_missing_webhook_preserves_summary(self):
        os.environ['FEISHU_WEBHOOK_URL'] = ''
        with self.assertRaisesRegex(ValueError, 'Configure'):
            review.notify(self.run)
        self.assertIn('审核结果', Path(os.environ['GITHUB_STEP_SUMMARY']).read_text())

    def test_attempt_pagination_and_exact_pr_comparison(self):
        run = dict(self.run, id=7, pull_requests=[{'base': {'sha': 'b' * 40},
                                                 'head': {'sha': 'c' * 40}}])
        os.environ['GITHUB_OUTPUT'] = str(Path(self.directory.name) / 'output')
        responses = [{'jobs': [{'name': 'test'}] * 100}, {'jobs': []}, {'files': [{'filename': 'services/core/main.go'}]}]
        with patch.object(review, 'api', side_effect=responses) as api:
            review.collect(run)
        self.assertEqual(api.call_args_list[0].args[0],
                         'actions/runs/7/attempts/2/jobs?per_page=100&page=1')
        self.assertEqual(api.call_args_list[2].args[0], f"compare/{'b' * 40}...{'c' * 40}")
        prompt = Path(os.environ['GITHUB_OUTPUT']).read_text()
        self.assertIn('ready=true', prompt)
        self.assertIn('--- docs/development.md ---', prompt)
        self.assertIn('--- services/core/IMPLEMENTATION.md ---', prompt)
        self.assertNotIn('--- apps/web/DESIGN.md ---', prompt)


if __name__ == '__main__':
    unittest.main()
