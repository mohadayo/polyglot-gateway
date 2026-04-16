import os
import logging
import datetime

from flask import Flask, request, jsonify
import jwt

app = Flask(__name__)

LOG_LEVEL = os.environ.get("LOG_LEVEL", "INFO").upper()
logging.basicConfig(
    level=getattr(logging, LOG_LEVEL, logging.INFO),
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
)
logger = logging.getLogger("auth-service")

SECRET_KEY = os.environ.get("JWT_SECRET", "dev-secret-key")
TOKEN_EXPIRY_MINUTES = int(os.environ.get("TOKEN_EXPIRY_MINUTES", "60"))
PORT = int(os.environ.get("AUTH_SERVICE_PORT", "8001"))

# In-memory user store for demonstration
USERS: dict[str, dict] = {}


@app.route("/health", methods=["GET"])
def health():
    logger.debug("Health check requested")
    return jsonify({"status": "healthy", "service": "auth-service"}), 200


@app.route("/register", methods=["POST"])
def register():
    data = request.get_json()
    if not data or "username" not in data or "password" not in data:
        logger.warning("Registration attempt with missing fields")
        return jsonify({"error": "username and password are required"}), 400

    username = data["username"]
    if username in USERS:
        logger.warning("Registration attempt for existing user: %s", username)
        return jsonify({"error": "user already exists"}), 409

    USERS[username] = {"password": data["password"]}
    logger.info("User registered: %s", username)
    return jsonify({"message": "user registered successfully"}), 201


@app.route("/login", methods=["POST"])
def login():
    data = request.get_json()
    if not data or "username" not in data or "password" not in data:
        logger.warning("Login attempt with missing fields")
        return jsonify({"error": "username and password are required"}), 400

    username = data["username"]
    user = USERS.get(username)
    if not user or user["password"] != data["password"]:
        logger.warning("Failed login for user: %s", username)
        return jsonify({"error": "invalid credentials"}), 401

    payload = {
        "sub": username,
        "iat": datetime.datetime.now(datetime.timezone.utc),
        "exp": datetime.datetime.now(datetime.timezone.utc)
        + datetime.timedelta(minutes=TOKEN_EXPIRY_MINUTES),
    }
    token = jwt.encode(payload, SECRET_KEY, algorithm="HS256")
    logger.info("User logged in: %s", username)
    return jsonify({"token": token}), 200


@app.route("/verify", methods=["POST"])
def verify():
    auth_header = request.headers.get("Authorization", "")
    if not auth_header.startswith("Bearer "):
        logger.warning("Verify request with missing or invalid Authorization header")
        return jsonify({"error": "Bearer token required"}), 401

    token = auth_header.split(" ", 1)[1]
    try:
        payload = jwt.decode(token, SECRET_KEY, algorithms=["HS256"])
        logger.info("Token verified for user: %s", payload.get("sub"))
        return jsonify({"valid": True, "user": payload["sub"]}), 200
    except jwt.ExpiredSignatureError:
        logger.warning("Expired token presented")
        return jsonify({"error": "token expired"}), 401
    except jwt.InvalidTokenError:
        logger.warning("Invalid token presented")
        return jsonify({"error": "invalid token"}), 401


if __name__ == "__main__":
    logger.info("Starting auth-service on port %d", PORT)
    app.run(host="0.0.0.0", port=PORT)
