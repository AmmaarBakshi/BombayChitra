"""Python client for the BombayChitra API: the open map of the Mumbai Metropolitan Region.

    from bombaychitra import BombayChitra
    api = BombayChitra("http://localhost:8080")
    print(api.route("19.1197,72.8464", "18.9400,72.8355")["duration_s"])
"""
from .client import BombayChitra, BombayChitraError

__all__ = ["BombayChitra", "BombayChitraError"]
__version__ = "0.1.0"
