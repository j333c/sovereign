import { describe, it } from "node:test";
import assert from "node:assert/strict";
import { SovereignApiClient, ProblemDetailsError } from "./client";

describe("SovereignApiClient", () => {
  it("successfully fetches getHealth with typed response", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      assert.equal(url, "http://localhost:8080/api/v1/health");

      return new Response(
        JSON.stringify({
          status: "ok",
          version: "0.1.0",
          database: "connected",
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const health = await client.getHealth();
    assert.equal(health.status, "ok");
    assert.equal(health.version, "0.1.0");
    assert.equal(health.database, "connected");
  });

  it("successfully fetches getBootStatus with typed response", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      assert.equal(url, "http://localhost:8080/api/v1/system/boot-status");

      return new Response(
        JSON.stringify({
          mode: "operational",
          clean_shutdown: true,
          diagnostics_completed: true,
          message: "All systems operational",
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const status = await client.getBootStatus();
    assert.equal(status.mode, "operational");
    assert.equal(status.clean_shutdown, true);
    assert.equal(status.diagnostics_completed, true);
    assert.equal(status.message, "All systems operational");
  });

  it("fetches getBootStatus in diagnostics mode post-power outage with reassurance copy", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          mode: "diagnostics",
          clean_shutdown: false,
          diagnostics_completed: false,
          message: "Checking system safety after power restoration... your files are secure.",
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const status = await client.getBootStatus();
    assert.equal(status.mode, "diagnostics");
    assert.equal(status.clean_shutdown, false);
    assert.equal(status.diagnostics_completed, false);
    assert.equal(status.message, "Checking system safety after power restoration... your files are secure.");
  });

  it("handles storage degraded HTTP 503 on boot-status probe", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          type: "https://sovereign.local/errors/http-503",
          title: "Storage Degraded",
          status: 503,
          detail: "State storage filesystem is unwritable or read-only",
          instance: "/api/v1/system/boot-status",
        }),
        {
          status: 503,
          headers: { "Content-Type": "application/problem+json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.getBootStatus();
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 503);
        assert.equal(err.title, "Storage Degraded");
        assert.equal(err.detail, "State storage filesystem is unwritable or read-only");
        return true;
      }
    );
  });

  it("successfully polls boot status from diagnostics to operational", async () => {
    let callCount = 0;
    const mockFetch: typeof fetch = async () => {
      callCount++;
      if (callCount === 1) {
        return new Response(
          JSON.stringify({
            mode: "diagnostics",
            clean_shutdown: false,
            diagnostics_completed: false,
            message: "Checking system safety after power restoration... your files are secure.",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      return new Response(
        JSON.stringify({
          mode: "operational",
          clean_shutdown: false,
          diagnostics_completed: true,
          message: "All systems operational",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const status1 = await client.getBootStatus();
    assert.equal(status1.mode, "diagnostics");
    assert.equal(status1.diagnostics_completed, false);

    const status2 = await client.getBootStatus();
    assert.equal(status2.mode, "operational");
    assert.equal(status2.diagnostics_completed, true);
    assert.equal(callCount, 2);
  });

  it("successfully fetches getIngressStatus with typed response", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      assert.equal(url, "http://localhost:8080/api/v1/system/ingress");

      return new Response(
        JSON.stringify({
          status: "ok",
          caddy_connected: true,
          mdns_active: true,
          hostname: "sovereign.local",
          routes: [
            { service: "web", subpath: "/", upstream: "sovereign-web:3000" },
            { service: "web", subpath: "/drop", upstream: "sovereign-web:3000" },
            { service: "daemon", subpath: "/api/*", upstream: "sovereign-daemon:8080" },
          ],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const ingress = await client.getIngressStatus();
    assert.equal(ingress.status, "ok");
    assert.equal(ingress.caddy_connected, true);
    assert.equal(ingress.mdns_active, true);
    assert.equal(ingress.hostname, "sovereign.local");
    assert.equal(ingress.routes.length, 3);
    assert.equal(ingress.routes[0].upstream, "sovereign-web:3000");
  });

  it("throws typed ProblemDetailsError on HTTP 503 problem response", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          type: "https://sovereign.local/errors/database-unavailable",
          title: "Database Unavailable",
          status: 503,
          detail: "SQLite WAL database failed health probe.",
          instance: "/api/v1/health",
        }),
        {
          status: 503,
          headers: { "Content-Type": "application/problem+json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.getHealth();
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 503);
        assert.equal(err.title, "Database Unavailable");
        assert.equal(err.detail, "SQLite WAL database failed health probe.");
        assert.equal(err.instance, "/api/v1/health");
        return true;
      }
    );
  });

  it("handles non-JSON error responses gracefully as ProblemDetailsError", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response("Bad Gateway", {
        status: 502,
        statusText: "Bad Gateway",
      });
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.getHealth();
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 502);
        assert.equal(err.title, "Bad Gateway");
        return true;
      }
    );
  });

  it("normalizes trailing slashes in baseUrl", async () => {
    let requestedUrl = "";
    const mockFetch: typeof fetch = async (input) => {
      requestedUrl = String(input);
      return new Response(JSON.stringify({ status: "ok", version: "0.1.0", database: "connected" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080///",
      fetchFn: mockFetch,
    });

    assert.equal(client.baseUrl, "http://localhost:8080");
    await client.getHealth();
    assert.equal(requestedUrl, "http://localhost:8080/api/v1/health");
  });

  it("defaults baseUrl to http://localhost:8080 in node environment when options are omitted", () => {
    const client = new SovereignApiClient();
    assert.equal(client.baseUrl, "http://localhost:8080");
  });

  it("safely falls back when receiving non-RFC7807 JSON error response", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          error: "Unauthorized action",
        }),
        {
          status: 401,
          statusText: "Unauthorized",
          headers: { "Content-Type": "application/json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.getHealth();
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 401);
        assert.equal(err.title, "Unauthorized");
        assert.equal(err.detail, "Unauthorized action");
        assert.equal(err.instance, "/api/v1/health");
        return true;
      }
    );
  });

  it("successfully fetches getDisks with typed response", async () => {
    const mockFetch: typeof fetch = async (input) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/storage/disks");
      return new Response(
        JSON.stringify({
          disks: [
            {
              name: "nvme0n1",
              path: "/dev/nvme0n1",
              model: "Samsung SSD 980 1TB",
              size_bytes: 1000204886016,
              type: "nvme",
              rotational: false,
              mounted: false,
              is_system: false,
            },
          ],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.getDisks();
    assert.equal(res.disks.length, 1);
    assert.equal(res.disks[0].name, "nvme0n1");
    assert.equal(res.disks[0].size_bytes, 1000204886016);
  });

  it("successfully calls initializeStorage with POST payload", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/storage/initialize");
      assert.equal(init?.method, "POST");
      assert.equal(init?.body, JSON.stringify({ device: "/dev/nvme0n1" }));

      return new Response(
        JSON.stringify({
          status: "initialized",
          device: "/dev/nvme0n1",
          mount_point: "/data",
          message: "Storage pool formatted and mounted successfully",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.initializeStorage({ device: "/dev/nvme0n1" });
    assert.equal(res.status, "initialized");
    assert.equal(res.mount_point, "/data");
  });

  it("successfully calls testNAS and returns success verification", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/storage/nas/test");
      assert.equal(init?.method, "POST");

      return new Response(
        JSON.stringify({
          success: true,
          message: "Connection and credentials verified successfully",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.testNAS({
      type: "smb",
      host: "192.168.1.100",
      share: "media",
      username: "user",
      password: "pass",
    });
    assert.equal(res.success, true);
  });

  it("successfully calls mountNAS and saves mount configuration", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/storage/nas/mount");
      assert.equal(init?.method, "POST");

      return new Response(
        JSON.stringify({
          status: "mounted",
          type: "smb",
          host: "192.168.1.100",
          share: "media",
          mount_point: "/data/media",
          message: "Network storage share mounted successfully",
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.mountNAS({
      type: "smb",
      host: "192.168.1.100",
      share: "media",
    });
    assert.equal(res.status, "mounted");
    assert.equal(res.mount_point, "/data/media");
  });

  it("successfully fetches getStorageStatus with redacted password and degraded notice", async () => {
    const mockFetch: typeof fetch = async (input) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/storage/status");
      return new Response(
        JSON.stringify({
          status: "degraded",
          pool_type: "nas",
          mount_point: "/data",
          total_bytes: 1000000000000,
          available_bytes: 800000000000,
          used_bytes: 200000000000,
          nas: {
            configured: true,
            type: "smb",
            host: "nas.local",
            share: "media",
            username: "alex",
            password: "***",
            mount_point: "/data/media",
            status: "degraded",
            message: "Network storage temporarily unavailable",
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const status = await client.getStorageStatus();
    assert.equal(status.status, "degraded");
    assert.equal(status.nas?.password, "***");
    assert.equal(status.nas?.message, "Network storage temporarily unavailable");
  });

  it("handles storage API ProblemDetailsError on invalid disk format", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          type: "https://sovereign.local/errors/http-400",
          title: "Invalid Device",
          status: 400,
          detail: "device contains root or boot partitions and cannot be formatted",
          instance: "/api/v1/storage/initialize",
        }),
        {
          status: 400,
          headers: { "Content-Type": "application/problem+json" },
        }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.initializeStorage({ device: "/dev/sda" });
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 400);
        assert.equal(err.title, "Invalid Device");
        assert.equal(err.detail, "device contains root or boot partitions and cannot be formatted");
        return true;
      }
    );
  });

  it("successfully fetches getSetupStatus with typed response", async () => {
    const mockFetch: typeof fetch = async (input) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/auth/setup-status");
      return new Response(JSON.stringify({ is_configured: false }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.getSetupStatus();
    assert.equal(res.is_configured, false);
  });

  it("successfully calls completeSetup and returns session", async () => {
    const mockFetch: typeof fetch = async (input, init) => {
      assert.equal(String(input), "http://localhost:8080/api/v1/auth/setup");
      assert.equal(init?.method, "POST");
      const body = JSON.parse(String(init?.body));
      assert.equal(body.password, "HouseholdMasterKey2026!");
      assert.equal(body.email, "alex@family.net");

      return new Response(
        JSON.stringify({
          status: "ok",
          is_configured: true,
          session: {
            token: "tok123",
            username: "admin",
            role: "admin",
            expires_at: 1730000000,
          },
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const res = await client.completeSetup({
      username: "admin",
      password: "HouseholdMasterKey2026!",
      email: "alex@family.net",
      language: "de",
      features: ["video", "audio"],
    });

    assert.equal(res.status, "ok");
    assert.equal(res.is_configured, true);
    assert.equal(res.session?.role, "admin");
    assert.equal(res.session?.token, "tok123");
  });

  it("handles duplicate setup error as ProblemDetailsError with 409 Conflict", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(
        JSON.stringify({
          type: "https://sovereign.local/errors/http-409",
          title: "Already Configured",
          status: 409,
          detail: "Appliance first-run setup has already been completed",
          instance: "/api/v1/auth/setup",
        }),
        { status: 409, headers: { "Content-Type": "application/problem+json" } }
      );
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    await assert.rejects(
      async () => {
        await client.completeSetup({
          password: "validPassword123!",
          email: "alex@family.net",
        });
      },
      (err: unknown) => {
        assert.ok(err instanceof ProblemDetailsError);
        assert.equal(err.status, 409);
        assert.equal(err.title, "Already Configured");
        return true;
      }
    );
  });

  it("successfully calls login, logout, and getSession", async () => {
    const calls: string[] = [];
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      calls.push(`${init?.method ?? "GET"} ${url}`);

      if (url.endsWith("/login")) {
        return new Response(
          JSON.stringify({
            status: "ok",
            session: {
              token: "session-abc",
              username: "admin",
              role: "admin",
              expires_at: 1730000000,
            },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url.endsWith("/session")) {
        return new Response(
          JSON.stringify({
            authenticated: true,
            session: {
              token: "session-abc",
              username: "admin",
              role: "admin",
              expires_at: 1730000000,
            },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      if (url.endsWith("/logout")) {
        return new Response(
          JSON.stringify({ status: "ok", message: "Logged out successfully" }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }

      throw new Error(`Unexpected endpoint: ${url}`);
    };

    const client = new SovereignApiClient({
      baseUrl: "http://localhost:8080",
      fetchFn: mockFetch,
    });

    const loginRes = await client.login({ username: "admin", password: "HouseholdMasterKey2026!" });
    assert.equal(loginRes.session?.token, "session-abc");

    const sessionRes = await client.getSession();
    assert.equal(sessionRes.authenticated, true);
    assert.equal(sessionRes.session?.username, "admin");

    const logoutRes = await client.logout();
    assert.equal(logoutRes.status, "ok");

    assert.equal(calls.length, 3);
  });

  it("always sends credentials: include on API requests", async () => {
    let capturedCredentials: RequestCredentials | undefined;
    const mockFetch: typeof fetch = async (input, init) => {
      capturedCredentials = init?.credentials;
      return new Response(JSON.stringify({ status: "ok", version: "0.1.0", database: "connected" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };

    const client = new SovereignApiClient({ baseUrl: "http://localhost:8080", fetchFn: mockFetch });
    await client.getHealth();
    assert.equal(capturedCredentials, "include");
  });

  it("safely handles HTTP 204 No Content or empty responses", async () => {
    const mockFetch: typeof fetch = async () => {
      return new Response(null, { status: 204 });
    };

    const client = new SovereignApiClient({ baseUrl: "http://localhost:8080", fetchFn: mockFetch });
    const res = await client.logout();
    assert.deepEqual(res, {});
  });

  it("successfully calls getSettings and updateSettings", async () => {
    let updatedPayload: unknown = null;
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      if (url.endsWith("/settings") && init?.method === "POST") {
        updatedPayload = JSON.parse(String(init.body));
        return new Response(JSON.stringify({ status: "ok" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      if (url.endsWith("/settings")) {
        return new Response(
          JSON.stringify({
            language: "de",
            admin_email: "alex@family.net",
            hostname: "sovereign.local",
            features: "video,audio,dms,ai",
          }),
          { status: 200, headers: { "Content-Type": "application/json" } }
        );
      }
      throw new Error(`Unexpected endpoint: ${url}`);
    };

    const client = new SovereignApiClient({ baseUrl: "http://localhost:8080", fetchFn: mockFetch });
    const settings = await client.getSettings();
    assert.equal(settings.language, "de");
    assert.equal(settings.hostname, "sovereign.local");

    const updateRes = await client.updateSettings({ language: "en" });
    assert.equal(updateRes.status, "ok");
    assert.deepEqual(updatedPayload, { language: "en" });
  });

  it("successfully fetches getServices with typed response", async () => {
    const mockServices = [
      {
        id: "jellyfin",
        name: "Jellyfin",
        status: "inactive",
        image: "jellyfin/jellyfin:12.2.0",
        image_downloaded: false,
        mem_limit_bytes: 2147483648,
        mem_reservation_bytes: 1073741824,
      },
    ];

    const mockFetch: typeof fetch = async (input) => {
      const url = String(input);
      assert.ok(url.endsWith("/api/v1/services"));
      return new Response(JSON.stringify({ services: mockServices }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    };

    const client = new SovereignApiClient({ baseUrl: "http://localhost:8080", fetchFn: mockFetch });
    const res = await client.getServices();
    assert.equal(res.services.length, 1);
    assert.equal(res.services[0].id, "jellyfin");
    assert.equal(res.services[0].image_downloaded, false);
    assert.equal(res.services[0].status, "inactive");
  });

  it("successfully calls activateService, deactivateService, and restartService", async () => {
    const calls: { url: string; method: string }[] = [];
    const mockFetch: typeof fetch = async (input, init) => {
      const url = String(input);
      calls.push({ url, method: init?.method || "GET" });
      const id = url.split("/")[url.split("/").length - 2];
      const action = url.split("/")[url.split("/").length - 1];

      return new Response(
        JSON.stringify({
          id,
          name: "Jellyfin",
          status: action === "activate" ? "running" : action === "deactivate" ? "inactive" : "running",
          image: "jellyfin/jellyfin:12.2.0",
          image_downloaded: true,
          mem_limit_bytes: 2147483648,
          mem_reservation_bytes: 1073741824,
        }),
        { status: 200, headers: { "Content-Type": "application/json" } }
      );
    };

    const client = new SovereignApiClient({ baseUrl: "http://localhost:8080", fetchFn: mockFetch });
    const actRes = await client.activateService("jellyfin");
    assert.equal(actRes.id, "jellyfin");
    assert.equal(actRes.status, "running");

    const deactRes = await client.deactivateService("jellyfin");
    assert.equal(deactRes.id, "jellyfin");
    assert.equal(deactRes.status, "inactive");

    const restartRes = await client.restartService("jellyfin");
    assert.equal(restartRes.id, "jellyfin");
    assert.equal(restartRes.status, "running");

    assert.equal(calls.length, 3);
    assert.ok(calls[0].url.endsWith("/api/v1/services/jellyfin/activate"));
    assert.equal(calls[0].method, "POST");
    assert.ok(calls[1].url.endsWith("/api/v1/services/jellyfin/deactivate"));
    assert.equal(calls[1].method, "POST");
    assert.ok(calls[2].url.endsWith("/api/v1/services/jellyfin/restart"));
    assert.equal(calls[2].method, "POST");
  });
});

