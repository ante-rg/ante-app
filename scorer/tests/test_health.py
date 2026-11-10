from fastapi.testclient import TestClient

from scorer.app import app


def test_healthz_reports_ok_without_model():
    response = TestClient(app).get("/healthz")
    assert response.status_code == 200
    assert response.json() == {"status": "ok", "model_version": None}
