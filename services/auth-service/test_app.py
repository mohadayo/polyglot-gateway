import json
import pytest
from app import app, USERS


@pytest.fixture
def client():
    app.config["TESTING"] = True
    USERS.clear()
    with app.test_client() as c:
        yield c


def test_health(client):
    resp = client.get("/health")
    assert resp.status_code == 200
    data = json.loads(resp.data)
    assert data["status"] == "healthy"
    assert data["service"] == "auth-service"


def test_register_success(client):
    resp = client.post(
        "/register",
        json={"username": "alice", "password": "secret"},
    )
    assert resp.status_code == 201
    data = json.loads(resp.data)
    assert data["message"] == "user registered successfully"


def test_register_missing_fields(client):
    resp = client.post("/register", json={"username": "alice"})
    assert resp.status_code == 400


def test_register_duplicate(client):
    client.post("/register", json={"username": "alice", "password": "secret"})
    resp = client.post("/register", json={"username": "alice", "password": "secret"})
    assert resp.status_code == 409


def test_login_success(client):
    client.post("/register", json={"username": "bob", "password": "pass123"})
    resp = client.post("/login", json={"username": "bob", "password": "pass123"})
    assert resp.status_code == 200
    data = json.loads(resp.data)
    assert "token" in data


def test_login_invalid_credentials(client):
    client.post("/register", json={"username": "bob", "password": "pass123"})
    resp = client.post("/login", json={"username": "bob", "password": "wrong"})
    assert resp.status_code == 401


def test_login_missing_fields(client):
    resp = client.post("/login", json={})
    assert resp.status_code == 400


def test_verify_valid_token(client):
    client.post("/register", json={"username": "carol", "password": "pw"})
    login_resp = client.post("/login", json={"username": "carol", "password": "pw"})
    token = json.loads(login_resp.data)["token"]

    resp = client.post("/verify", headers={"Authorization": f"Bearer {token}"})
    assert resp.status_code == 200
    data = json.loads(resp.data)
    assert data["valid"] is True
    assert data["user"] == "carol"


def test_verify_missing_header(client):
    resp = client.post("/verify")
    assert resp.status_code == 401


def test_verify_invalid_token(client):
    resp = client.post("/verify", headers={"Authorization": "Bearer invalid.token.here"})
    assert resp.status_code == 401
