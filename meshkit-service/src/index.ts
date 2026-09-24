import express from "express";
import cors from "cors";
import dotenv from "dotenv";
import { init, setupGracefulShutdown } from "@ipfs-meshkit/meshkit";

dotenv.config();

const PORT = Number(process.env.PORT ?? 3001);
const ipfsRpcUrl = process.env.IPFS_RPC_URL || "http://ipfs:5001";

type MeshkitInstance = Awaited<ReturnType<typeof init>>["meshkit"];

let mesh: MeshkitInstance | undefined;

async function initializeMeshkit(): Promise<void> {
  try {
    const initialized = await init({ localNode: false, nodes: [ipfsRpcUrl] });
    mesh = initialized.meshkit;
    console.log(
      `Meshkit initialized; active nodes: ${initialized.meshkit.activeNodes.join(", ")}`
    );
    if (initialized.localNode !== undefined) {
      setupGracefulShutdown(initialized.localNode);
    }
  } catch (err) {
    mesh = undefined;
    console.error(
      `Meshkit initialization failed (node: ${ipfsRpcUrl}): ` +
        `${(err as Error).message}`
    );
    console.error(
      "File endpoints will return 503 until a reachable Kubo node is configured."
    );
  }
}

const app = express();
app.use(cors());
app.use(express.json({ limit: "50mb" }));

app.get("/health", (_req, res) => {
  res.json({ status: "ok" });
});

app.post("/files/put", async (req, res) => {
  try {
    if (!mesh) {
      res
        .status(503)
        .json({
          error: "Meshkit not initialized: no reachable Kubo node",
        });
      return;
    }
    const { path, content } = req.body ?? {};
    if (typeof path !== "string" || typeof content !== "string") {
      res
        .status(400)
        .json({ error: "request body must be JSON { path: string, content: string }" });
      return;
    }
    const bytes = new TextEncoder().encode(content);
    const cid = await mesh.upload(bytes);
    res.json({ cid, path });
  } catch (err) {
    res.status(500).json({ error: (err as Error).message });
  }
});

app.get("/files/*", async (req, res) => {
  try {
    if (!mesh) {
      res
        .status(503)
        .json({
          error: "Meshkit not initialized: no reachable Kubo node",
        });
      return;
    }
    const raw = (req.params as unknown as string[])[0] ?? "";
    const cid = raw?.split("/")[0];
    if (!cid) {
      res.status(400).json({ error: "no CID provided in path" });
      return;
    }
    const data = await mesh.retrieve(cid);
    res.set("Content-Type", "text/plain; charset=utf-8");
    res.send(new TextDecoder().decode(data));
  } catch (err) {
    res.status(404).json({ error: (err as Error).message });
  }
});

void initializeMeshkit();

app.listen(PORT, () => {
  console.log(`meshkit-service listening on http://localhost:${PORT}`);
});