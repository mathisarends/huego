import importlib.util
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("catalog", Path(__file__).with_name("update-api-catalog.py"))
catalog = importlib.util.module_from_spec(spec)
spec.loader.exec_module(catalog)


class CatalogTests(unittest.TestCase):
    def test_yaml_preserves_field_names_enums_and_booleans(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "schema.yaml"
            path.write_text("properties:\n  on:\n    type: boolean\n    default: true\nrequired: [on]\nenum: [on, off, true, false]\n", encoding="utf-8")
            schema = catalog.load_yaml(path)
            self.assertEqual(schema["required"], ["on"])
            self.assertIs(schema["properties"]["on"]["default"], True)
            self.assertEqual(schema["enum"], ["on", "off", True, False])

    def test_schema_retains_agent_guidance(self):
        schema = {"description": "milliseconds", "example": 1000, "properties": {"description": {"type": "string"}}}
        self.assertEqual(catalog.schema_facts(schema), schema)


if __name__ == "__main__":
    unittest.main()
