import subprocess
import unittest
from pathlib import Path

try:
    from deploy import validate
except ModuleNotFoundError:  # direct `python deploy/test_validate.py`
    import validate


class DeploymentValidatorTests(unittest.TestCase):
    def test_checked_in_bundle_passes(self):
        validate.validate()

    def test_compose_rejects_local_database_services(self):
        document = validate.load_compose()
        document["services"]["postgres"] = {}
        with self.assertRaises(validate.ValidationError):
            validate.validate_compose(document)

    def test_compose_requires_tls_for_managed_services(self):
        document = validate.load_compose()
        document["services"]["api"]["environment"]["DB_SSL_MODE"] = "disable"
        with self.assertRaises(validate.ValidationError):
            validate.validate_compose(document)

    def test_shell_hooks_parse_and_help(self):
        root = Path(__file__).resolve().parent
        for script in (
            root / "backup.sh",
            root / "verify-backup.sh",
            root / "rollback.sh",
            root / "monitor.sh",
            root / "vps-bootstrap.sh",
        ):
            subprocess.run(["bash", "-n", str(script)], check=True)
            result = subprocess.run([str(script), "--help"], check=True, capture_output=True, text=True)
            self.assertIn("Usage:", result.stdout)

    def test_default_services_are_direct_and_observability_is_opt_in(self):
        document = validate.load_compose()
        services = document["services"]
        self.assertEqual(set(services["api"]["environment"]) & {"ROUTE_TO_GO"}, {"ROUTE_TO_GO"})
        self.assertEqual(services["api"]["environment"]["ROUTE_TO_GO"], "true")
        self.assertNotIn("LEGACY_UPSTREAM", services["nginx"].get("environment", {}))
        for name in ("prometheus", "alertmanager", "grafana"):
            self.assertIn("observability", services[name]["profiles"])


if __name__ == "__main__":
    unittest.main()
