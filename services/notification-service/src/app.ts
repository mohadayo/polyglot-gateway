import express, { Request, Response } from "express";

const app = express();
app.use(express.json());

const PORT = parseInt(process.env.NOTIFICATION_SERVICE_PORT || "8003", 10);
const LOG_LEVEL = (process.env.LOG_LEVEL || "INFO").toUpperCase();

function log(level: string, message: string): void {
  const levels = ["DEBUG", "INFO", "WARN", "ERROR"];
  const configuredIdx = levels.indexOf(LOG_LEVEL);
  const msgIdx = levels.indexOf(level);
  if (msgIdx >= configuredIdx) {
    const timestamp = new Date().toISOString();
    console.log(`${timestamp} [${level}] notification-service: ${message}`);
  }
}

interface Notification {
  id: string;
  channel: string;
  recipient: string;
  subject: string;
  body: string;
  status: string;
  createdAt: string;
}

const notifications: Notification[] = [];
let idCounter = 0;

const VALID_CHANNELS = ["email", "sms", "webhook"];

app.get("/health", (_req: Request, res: Response) => {
  log("DEBUG", "Health check requested");
  res.json({ status: "healthy", service: "notification-service" });
});

app.post("/notify", (req: Request, res: Response) => {
  const { channel, recipient, subject, body } = req.body;

  if (!channel || !recipient || !body) {
    log("WARN", "Notify request with missing fields");
    res.status(400).json({ error: "channel, recipient, and body are required" });
    return;
  }

  if (!VALID_CHANNELS.includes(channel)) {
    log("WARN", `Invalid channel: ${channel}`);
    res
      .status(400)
      .json({ error: `invalid channel, must be one of: ${VALID_CHANNELS.join(", ")}` });
    return;
  }

  idCounter++;
  const notification: Notification = {
    id: `notif-${idCounter}`,
    channel,
    recipient,
    subject: subject || "",
    body,
    status: "sent",
    createdAt: new Date().toISOString(),
  };

  notifications.push(notification);
  log("INFO", `Notification sent: ${notification.id} via ${channel} to ${recipient}`);
  res.status(201).json(notification);
});

app.get("/notifications", (_req: Request, res: Response) => {
  log("DEBUG", `Listing ${notifications.length} notifications`);
  res.json({ notifications, total: notifications.length });
});

app.get("/notifications/:id", (req: Request, res: Response) => {
  const notif = notifications.find((n) => n.id === req.params.id);
  if (!notif) {
    log("WARN", `Notification not found: ${req.params.id}`);
    res.status(404).json({ error: "notification not found" });
    return;
  }
  res.json(notif);
});

export { app, notifications, idCounter };

if (require.main === module) {
  app.listen(PORT, "0.0.0.0", () => {
    log("INFO", `Starting notification-service on port ${PORT}`);
  });
}

export default app;
