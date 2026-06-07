#!/usr/bin/env python3
"""Tests for the post-deploy scenario acceptance script."""

from __future__ import annotations

import importlib.util
import pathlib
import sys
import unittest


SCRIPT_PATH = pathlib.Path(__file__).with_name("scenario_acceptance.py")
SPEC = importlib.util.spec_from_file_location("scenario_acceptance", SCRIPT_PATH)
scenario_acceptance = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
sys.modules[SPEC.name] = scenario_acceptance
SPEC.loader.exec_module(scenario_acceptance)


class FakeClient:
    def __init__(self) -> None:
        self.requests: list[dict] = []
        self.assignment_id = 19000

    def request(self, method: str, path: str, body: dict | None = None, **kwargs) -> dict:
        headers = kwargs.get("headers") or {}
        self.requests.append({"method": method.upper(), "path": path, "body": body or {}, "headers": headers})
        status = 200
        response = self.response_for(method.upper(), path, body or {}, headers)
        if isinstance(response, tuple):
            status, response = response
        return {"status": status, "ms": 1, "body": response}

    def response_for(self, method: str, path: str, body: dict, headers: dict) -> dict | tuple[int, dict]:
        if method == "GET" and path == "/api/order/form":
            return {
                "sources": [{"id": 1, "name": "微信"}],
                "order_types": [{"id": 2, "name": "批发"}],
                "employees": [{"id": 3, "name": "Codex"}],
                "pay_statuses": [{"id": 4, "name": "未付款"}],
                "ship_statuses": [{"id": 5, "name": "未发货"}],
            }
        if method == "POST" and path == "/api/customers":
            return {"customer": {"id": 147, "name": body.get("name", "")}}
        if method == "POST" and path == "/api/materials":
            return {"id": 53, "name": body.get("name", "")}
        if method == "POST" and path == "/api/product-settings/products":
            return {"product": {"id": 547, "name": body.get("name", "")}}
        if method == "POST" and path == "/api/business-groups":
            return {
                "group": {
                    "id": 77,
                    "name": body.get("name", ""),
                    "items": [
                        {
                            "id": 701,
                            "name": "Scenario Parent",
                            "children": [{"id": 702, "name": "Scenario Subgroup"}],
                        }
                    ],
                }
            }
        if method == "POST" and path == "/api/business-group-assignments":
            self.assignment_id += 1
            return {"assignment": {"id": self.assignment_id}}
        if method == "POST" and path == "/api/production-boms":
            return {"id": 3262, "latest_version_id": 900, "latest_version_no": "V001"}
        if path.startswith("/api/production-bom-versions/"):
            return {"ok": True}
        if method == "GET" and path == "/api/stock/warehouses":
            return {"rows": [{"code": "MAIN"}]}
        if method == "POST" and path == "/api/product-pricing-rules":
            return {"rule": {"id": 8, "code": body.get("code", ""), "formula_version": "v1", "calculation_json": {}}}
        if method == "POST" and path == "/api/price-tier-templates":
            return {"template": {"id": 8, "tiers": [{"id": 801, "pricing_rule_id": 8}]}}
        if method == "POST" and path == "/api/product-customer-references":
            return {
                "reference": {
                    "id": 8,
                    "customer_id": body.get("customer_id", 0),
                    "customer_display_name": body.get("customer_display_name", ""),
                    "customer_item_code": body.get("customer_item_code", ""),
                }
            }
        if method == "PUT" and path.startswith("/api/customer-portal/admin/capability-templates/"):
            return {"key": path.rsplit("/", 1)[-1], "capabilities": body.get("capabilities", []), "active": body.get("active", True)}
        if method == "PUT" and path == "/api/customer-portal/admin/customers/147/visibility":
            return {"customer": {"id": 147, "portal_enabled": body.get("enabled", True)}}
        if method == "POST" and path == "/api/customer-fulfillment/147/external-users":
            return {"customer_id": 147, "employee_id": 24, "phone": body.get("phone", ""), "login_enabled": True}
        if method == "POST" and path == "/api/mini/login/password":
            return {"token": "mini-token", "current_customer_id": 147, "capabilities": [{"code": "settlement", "enabled": True}]}
        if method == "POST" and path == "/api/costing/bean-list/publications":
            return {"id": 64, "version": body.get("version", "")}
        if method == "POST" and path == "/api/order":
            return {"order_id": 1531, "order_no": "SO-SCENARIO"}
        if method == "GET" and path == "/api/orders/1531/detail":
            return {
                "order": {"id": 1531, "order_no": "SO-SCENARIO", "grand_total": "88.00"},
                "quote_source_trace": {"version": "SCENARIO-PRICE"},
                "production_source_trace": {"bom_id": 3262},
            }
        if method == "GET" and path == "/api/orders/1531/sales-order-preview":
            return {"order_id": 1531}
        if method == "GET" and path == "/api/mini/customer-products":
            if headers.get("Authorization") != "Bearer mini-token":
                return 401, {"error": "mini token required"}
            return {"current_customer_id": 147, "products": []}
        if method == "GET" and path == "/api/mini/services/settlement":
            if headers.get("Authorization") != "Bearer mini-token":
                return 401, {"error": "mini token required"}
            return {
                "current_customer_id": 147,
                "orders": [{"id": 1531, "order_no": "SO-SCENARIO", "grand_total": "88.00"}],
                "summary": [{"label": "应收总额", "value": "88.00"}],
            }
        if method in {"POST", "PUT", "DELETE"}:
            return {"ok": True}
        return {}


class ScenarioAcceptanceTest(unittest.TestCase):
    def test_main_flow_logs_into_customer_miniapp_and_asserts_settlement_order(self) -> None:
        client = FakeClient()
        context = {
            "run_id": "PR442-SCENARIO-20260607-UNIT",
            "today": "2026-06-07",
            "created": {},
        }
        cleanup_stack: list = []
        results: list = []
        failures: list = []

        scenario_acceptance.run_generated_main_flow(client, context, cleanup_stack, results, failures)

        self.assertEqual([], failures)
        paths = [request["path"] for request in client.requests]
        self.assertIn("/api/customer-portal/admin/capability-templates/pr442_scenario_20260607_unit", paths)
        self.assertIn("/api/customer-portal/admin/customers/147/visibility", paths)
        self.assertIn("/api/customer-fulfillment/147/external-users", paths)
        self.assertIn("/api/mini/login/password", paths)

        mini_reads = [request for request in client.requests if request["path"].startswith("/api/mini/") and request["method"] == "GET"]
        self.assertTrue(mini_reads, "main flow should read customer miniapp endpoints")
        for request in mini_reads:
            self.assertEqual("Bearer mini-token", request["headers"].get("Authorization"))

        cleanup_names = [action.name for action in cleanup_stack]
        self.assertIn("disable generated external user login", cleanup_names)
        self.assertIn("disable generated customer portal visibility", cleanup_names)
        self.assertIn("deactivate generated customer capability template", cleanup_names)


if __name__ == "__main__":
    unittest.main()
