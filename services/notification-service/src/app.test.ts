import request from "supertest";
import { app, notifications } from "./app";

beforeEach(() => {
  notifications.length = 0;
});

describe("GET /health", () => {
  it("returns healthy status", async () => {
    const res = await request(app).get("/health");
    expect(res.status).toBe(200);
    expect(res.body.status).toBe("healthy");
    expect(res.body.service).toBe("notification-service");
  });
});

describe("POST /notify", () => {
  it("sends a notification successfully", async () => {
    const res = await request(app).post("/notify").send({
      channel: "email",
      recipient: "user@example.com",
      subject: "Test",
      body: "Hello World",
    });
    expect(res.status).toBe(201);
    expect(res.body.id).toBeDefined();
    expect(res.body.channel).toBe("email");
    expect(res.body.recipient).toBe("user@example.com");
    expect(res.body.status).toBe("sent");
  });

  it("sends an SMS notification", async () => {
    const res = await request(app).post("/notify").send({
      channel: "sms",
      recipient: "+1234567890",
      body: "Hello via SMS",
    });
    expect(res.status).toBe(201);
    expect(res.body.channel).toBe("sms");
  });

  it("sends a webhook notification", async () => {
    const res = await request(app).post("/notify").send({
      channel: "webhook",
      recipient: "https://example.com/hook",
      body: '{"event": "test"}',
    });
    expect(res.status).toBe(201);
    expect(res.body.channel).toBe("webhook");
  });

  it("rejects missing fields", async () => {
    const res = await request(app).post("/notify").send({
      channel: "email",
    });
    expect(res.status).toBe(400);
    expect(res.body.error).toContain("required");
  });

  it("rejects invalid channel", async () => {
    const res = await request(app).post("/notify").send({
      channel: "pigeon",
      recipient: "user@example.com",
      body: "test",
    });
    expect(res.status).toBe(400);
    expect(res.body.error).toContain("invalid channel");
  });
});

describe("GET /notifications", () => {
  it("returns empty list initially", async () => {
    const res = await request(app).get("/notifications");
    expect(res.status).toBe(200);
    expect(res.body.notifications).toEqual([]);
    expect(res.body.total).toBe(0);
  });

  it("returns notifications after sending", async () => {
    await request(app).post("/notify").send({
      channel: "email",
      recipient: "a@b.com",
      body: "test",
    });
    const res = await request(app).get("/notifications");
    expect(res.status).toBe(200);
    expect(res.body.total).toBe(1);
  });
});

describe("GET /notifications/:id", () => {
  it("returns a specific notification", async () => {
    const createRes = await request(app).post("/notify").send({
      channel: "email",
      recipient: "a@b.com",
      body: "specific test",
    });
    const id = createRes.body.id;
    const res = await request(app).get(`/notifications/${id}`);
    expect(res.status).toBe(200);
    expect(res.body.id).toBe(id);
    expect(res.body.body).toBe("specific test");
  });

  it("returns 404 for unknown notification", async () => {
    const res = await request(app).get("/notifications/notif-999");
    expect(res.status).toBe(404);
    expect(res.body.error).toBe("notification not found");
  });
});
