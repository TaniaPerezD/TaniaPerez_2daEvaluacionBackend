

// ─── BOOTSTRAP & TERMINATION HOOKS ──────────────────────────────────────

onBootstrap(function(e) {
    // pb_hooks/pb.js

// ─── GLOBAL CONFIG & HELPERS ────────────────────────────────────────────

var APP         = "POCKETBASE-CENTRAL";
var HOST        = "localhost";
var IP          = "127.0.0.1";
var PORT        = 3000;
var INSTANCE_ID = HOST + ":" + APP + ":" + PORT;
var BASE_URL    = "http://172.25.136.15:8761/eureka";

var URLS = {
  register:   BASE_URL + "/apps/" + APP,
  heartbeat:  BASE_URL + "/apps/" + APP + "/" + INSTANCE_ID,
  deregister: BASE_URL + "/apps/" + APP + "/" + INSTANCE_ID,
  apps:       BASE_URL + "/apps"
};

function register() {
  try {
    var res = $http.send({
      url:     URLS.register,
      method:  "POST",
      headers: { "Content-Type": "application/json" },
      body:    JSON.stringify({
        instance: {
          instanceId:   INSTANCE_ID,
          hostName:     HOST,
          app:          APP,
          ipAddr:       IP,
          status:       "UP",
          port:         { $: PORT, "@enabled": true },
          vipAddress:   APP.toLowerCase(),
          dataCenterInfo: {
            "@class": "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
            name:     "MyOwn"
          }
        }
      })
    });

    if (res.statusCode === 204) {
      console.log("registration successful");
    } else {
      console.log("registration failed", "statusCode", res.statusCode);
    }
  } catch (err) {
    console.log("registration error", err);
  }
}

function sendHeartbeat() {
  try {
    var res = $http.send({ url: URLS.heartbeat, method: "PUT" });
    if (res.statusCode === 200) {
      console.log("heartbeat successful");
    } else {
      console.log("heartbeat unexpected status", "statusCode", res.statusCode);
    }
  } catch (err) {
    console.log("heartbeat error", err);
  }
}

function discoverAndPing() {
  try {
    var res = $http.send({
      url:     URLS.apps,
      method:  "GET",
      headers: { Accept: "application/json" }
    });
    if (res.statusCode !== 200) {
      console.log("failed to fetch apps list", "statusCode", res.statusCode);
      return;
    }

    var appsNode = res.json.applications && res.json.applications.application;
    var appsArr  = Array.isArray(appsNode) ? appsNode : appsNode ? [appsNode] : [];

    appsArr.forEach(function(appEntry) {
      var insts = Array.isArray(appEntry.instance)
        ? appEntry.instance
        : [appEntry.instance];

      insts.forEach(function(inst) {
        var target = "http://" + inst.ipAddr + ":" + inst.port["$"] + "/";
        try {
          var pingRes = $http.send({ url: target, method: "GET", timeout: 5 });
          console.log(
            "ping successful",
            "targetApp",   appEntry.name,
            "instanceId",  inst.instanceId,
            "statusCode",  pingRes.statusCode
          );
        } catch (pingErr) {
          console.log(
            "ping failed",
            "targetApp",   appEntry.name,
            "instanceId",  inst.instanceId,
            "error",       pingErr
          );
        }
      });
    });
  } catch (err) {
    console.log("discovery error", err);
  }
}

  e.next();

  // initial registration
  register();

  // schedule cron jobs by passing global functions
  cronAdd("eureka-heartbeat", "*/1 * * * *", sendHeartbeat);
  cronAdd("eureka-discovery", "*/5 * * * *", discoverAndPing);

  console.log("scheduled heartbeat & discovery crons");
});

onTerminate(function(e) {
  try {
    cronRemove("eureka-heartbeat");
    cronRemove("eureka-discovery");
    console.log("removed heartbeat & discovery crons");
  } catch (err) {
    console.log("cron cleanup error", err);
  }

  try {
    var res = $http.send({ url: URLS.deregister, method: "DELETE" });
    if (res.statusCode === 200) {
      console.log("deregistration successful");
    } else {
      console.log("deregistration failed", "statusCode", res.statusCode);
    }
  } catch (err) {
    console.log("deregistration error", err);
  }
});
