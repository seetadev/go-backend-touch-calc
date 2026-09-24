"use strict";
var __importDefault = (this && this.__importDefault) || function (mod) {
    return (mod && mod.__esModule) ? mod : { "default": mod };
};
Object.defineProperty(exports, "__esModule", { value: true });
const express_1 = __importDefault(require("express"));
const cors_1 = __importDefault(require("cors"));
const dotenv_1 = __importDefault(require("dotenv"));
const meshkit_1 = require("@ipfs-meshkit/meshkit");
dotenv_1.default.config();
const PORT = Number(process.env.PORT ?? 3001);
const ipfsRpcUrl = process.env.IPFS_RPC_URL || "http://ipfs:5001";
let mesh;
async function initializeMeshkit() {
    try {
        const initialized = await (0, meshkit_1.init)({ localNode: false, nodes: [ipfsRpcUrl] });
        mesh = initialized.meshkit;
        console.log(`Meshkit initialized; active nodes: ${initialized.meshkit.activeNodes.join(", ")}`);
        if (initialized.localNode !== undefined) {
            (0, meshkit_1.setupGracefulShutdown)(initialized.localNode);
        }
    }
    catch (err) {
        mesh = undefined;
        console.error(`Meshkit initialization failed (node: ${ipfsRpcUrl}): ` +
            `${err.message}`);
        console.error("File endpoints will return 503 until a reachable Kubo node is configured.");
    }
}
const app = (0, express_1.default)();
app.use((0, cors_1.default)());
app.use(express_1.default.json({ limit: "50mb" }));
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
    }
    catch (err) {
        res.status(500).json({ error: err.message });
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
        const raw = req.params[0] ?? "";
        const cid = raw?.split("/")[0];
        if (!cid) {
            res.status(400).json({ error: "no CID provided in path" });
            return;
        }
        const data = await mesh.retrieve(cid);
        res.set("Content-Type", "text/plain; charset=utf-8");
        res.send(new TextDecoder().decode(data));
    }
    catch (err) {
        res.status(404).json({ error: err.message });
    }
});
void initializeMeshkit();
app.listen(PORT, () => {
    console.log(`meshkit-service listening on http://localhost:${PORT}`);
});
