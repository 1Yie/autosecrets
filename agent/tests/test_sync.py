"""Unit tests for the server-advertised poll interval handling in sync."""

import pytest

from autosecrets_agent.sync import _interval_from


@pytest.mark.parametrize(
    "body,expected",
    [
        ({"poll_interval_seconds": 15}, 15.0),
        ({"poll_interval_seconds": 60.0}, 60.0),
        ({"poll_interval_seconds": 5}, 5.0),
        ({"poll_interval_seconds": 86400}, 86400.0),
        ({}, None),
        ({"poll_interval_seconds": None}, None),
        ({"poll_interval_seconds": "15"}, None),
        ({"poll_interval_seconds": True}, None),
        ({"poll_interval_seconds": 4}, None),
        ({"poll_interval_seconds": 86401}, None),
        ({"poll_interval_seconds": -10}, None),
        (None, None),
        ({"status": "ok"}, None),
    ],
)
def test_interval_from(body, expected):
    assert _interval_from(body) == expected
