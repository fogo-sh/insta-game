"use strict";
var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __commonJS = (cb, mod) => function __require() {
  return mod || (0, cb[__getOwnPropNames(cb)[0]])((mod = { exports: {} }).exports, mod), mod.exports;
};
var __export = (target, all) => {
  for (var name in all)
    __defProp(target, name, { get: all[name], enumerable: true });
};
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
  mod
));
var __toCommonJS = (mod) => __copyProps(__defProp({}, "__esModule", { value: true }), mod);

// node_modules/discord-interactions/dist/util.js
var require_util = __commonJS({
  "node_modules/discord-interactions/dist/util.js"(exports2) {
    "use strict";
    Object.defineProperty(exports2, "__esModule", { value: true });
    exports2.concatUint8Arrays = exports2.valueToUint8Array = exports2.subtleCrypto = void 0;
    function getSubtleCrypto() {
      if (typeof window !== "undefined" && window.crypto) {
        return window.crypto.subtle;
      }
      if (typeof globalThis !== "undefined" && globalThis.crypto) {
        return globalThis.crypto.subtle;
      }
      if (typeof crypto !== "undefined") {
        return crypto.subtle;
      }
      if (typeof require === "function") {
        const cryptoPackage = "node:crypto";
        const crypto2 = require(cryptoPackage);
        return crypto2.webcrypto.subtle;
      }
      throw new Error("No Web Crypto API implementation found");
    }
    exports2.subtleCrypto = getSubtleCrypto();
    function valueToUint8Array(value, format) {
      if (value == null) {
        return new Uint8Array();
      }
      if (typeof value === "string") {
        if (format === "hex") {
          const matches = value.match(/.{1,2}/g);
          if (matches == null) {
            throw new Error("Value is not a valid hex string");
          }
          const hexVal = matches.map((byte) => Number.parseInt(byte, 16));
          return new Uint8Array(hexVal);
        }
        return new TextEncoder().encode(value);
      }
      try {
        if (Buffer.isBuffer(value)) {
          return new Uint8Array(value);
        }
      } catch (_ex) {
      }
      if (value instanceof ArrayBuffer) {
        return new Uint8Array(value);
      }
      if (value instanceof Uint8Array) {
        return value;
      }
      throw new Error("Unrecognized value type, must be one of: string, Buffer, ArrayBuffer, Uint8Array");
    }
    exports2.valueToUint8Array = valueToUint8Array;
    function concatUint8Arrays(arr1, arr2) {
      const merged = new Uint8Array(arr1.length + arr2.length);
      merged.set(arr1);
      merged.set(arr2, arr1.length);
      return merged;
    }
    exports2.concatUint8Arrays = concatUint8Arrays;
  }
});

// node_modules/discord-interactions/dist/webhooks.js
var require_webhooks = __commonJS({
  "node_modules/discord-interactions/dist/webhooks.js"(exports2) {
    "use strict";
    Object.defineProperty(exports2, "__esModule", { value: true });
    exports2.WebhookEventType = exports2.WebhookType = void 0;
    var WebhookType;
    (function(WebhookType2) {
      WebhookType2[WebhookType2["PING"] = 0] = "PING";
      WebhookType2[WebhookType2["EVENT"] = 1] = "EVENT";
    })(WebhookType || (exports2.WebhookType = WebhookType = {}));
    var WebhookEventType;
    (function(WebhookEventType2) {
      WebhookEventType2["APPLICATION_AUTHORIZED"] = "APPLICATION_AUTHORIZED";
      WebhookEventType2["APPLICATION_DEAUTHORIZED"] = "APPLICATION_DEAUTHORIZED";
      WebhookEventType2["ENTITLEMENT_CREATE"] = "ENTITLEMENT_CREATE";
      WebhookEventType2["QUEST_USER_ENROLLMENT"] = "QUEST_USER_ENROLLMENT";
      WebhookEventType2["LOBBY_MESSAGE_CREATE"] = "LOBBY_MESSAGE_CREATE";
      WebhookEventType2["LOBBY_MESSAGE_UPDATE"] = "LOBBY_MESSAGE_UPDATE";
      WebhookEventType2["LOBBY_MESSAGE_DELETE"] = "LOBBY_MESSAGE_DELETE";
      WebhookEventType2["GAME_DIRECT_MESSAGE_CREATE"] = "GAME_DIRECT_MESSAGE_CREATE";
      WebhookEventType2["GAME_DIRECT_MESSAGE_UPDATE"] = "GAME_DIRECT_MESSAGE_UPDATE";
      WebhookEventType2["GAME_DIRECT_MESSAGE_DELETE"] = "GAME_DIRECT_MESSAGE_DELETE";
    })(WebhookEventType || (exports2.WebhookEventType = WebhookEventType = {}));
  }
});

// node_modules/discord-interactions/dist/components.js
var require_components = __commonJS({
  "node_modules/discord-interactions/dist/components.js"(exports2) {
    "use strict";
    Object.defineProperty(exports2, "__esModule", { value: true });
    exports2.SeparatorSpacingTypes = exports2.TextStyleTypes = exports2.ChannelTypes = exports2.ButtonStyleTypes = exports2.MessageComponentTypes = void 0;
    var MessageComponentTypes;
    (function(MessageComponentTypes2) {
      MessageComponentTypes2[MessageComponentTypes2["ACTION_ROW"] = 1] = "ACTION_ROW";
      MessageComponentTypes2[MessageComponentTypes2["BUTTON"] = 2] = "BUTTON";
      MessageComponentTypes2[MessageComponentTypes2["STRING_SELECT"] = 3] = "STRING_SELECT";
      MessageComponentTypes2[MessageComponentTypes2["INPUT_TEXT"] = 4] = "INPUT_TEXT";
      MessageComponentTypes2[MessageComponentTypes2["USER_SELECT"] = 5] = "USER_SELECT";
      MessageComponentTypes2[MessageComponentTypes2["ROLE_SELECT"] = 6] = "ROLE_SELECT";
      MessageComponentTypes2[MessageComponentTypes2["MENTIONABLE_SELECT"] = 7] = "MENTIONABLE_SELECT";
      MessageComponentTypes2[MessageComponentTypes2["CHANNEL_SELECT"] = 8] = "CHANNEL_SELECT";
      MessageComponentTypes2[MessageComponentTypes2["SECTION"] = 9] = "SECTION";
      MessageComponentTypes2[MessageComponentTypes2["TEXT_DISPLAY"] = 10] = "TEXT_DISPLAY";
      MessageComponentTypes2[MessageComponentTypes2["THUMBNAIL"] = 11] = "THUMBNAIL";
      MessageComponentTypes2[MessageComponentTypes2["MEDIA_GALLERY"] = 12] = "MEDIA_GALLERY";
      MessageComponentTypes2[MessageComponentTypes2["FILE"] = 13] = "FILE";
      MessageComponentTypes2[MessageComponentTypes2["SEPARATOR"] = 14] = "SEPARATOR";
      MessageComponentTypes2[MessageComponentTypes2["CONTAINER"] = 17] = "CONTAINER";
      MessageComponentTypes2[MessageComponentTypes2["LABEL"] = 18] = "LABEL";
    })(MessageComponentTypes || (exports2.MessageComponentTypes = MessageComponentTypes = {}));
    var ButtonStyleTypes;
    (function(ButtonStyleTypes2) {
      ButtonStyleTypes2[ButtonStyleTypes2["PRIMARY"] = 1] = "PRIMARY";
      ButtonStyleTypes2[ButtonStyleTypes2["SECONDARY"] = 2] = "SECONDARY";
      ButtonStyleTypes2[ButtonStyleTypes2["SUCCESS"] = 3] = "SUCCESS";
      ButtonStyleTypes2[ButtonStyleTypes2["DANGER"] = 4] = "DANGER";
      ButtonStyleTypes2[ButtonStyleTypes2["LINK"] = 5] = "LINK";
      ButtonStyleTypes2[ButtonStyleTypes2["PREMIUM"] = 6] = "PREMIUM";
    })(ButtonStyleTypes || (exports2.ButtonStyleTypes = ButtonStyleTypes = {}));
    var ChannelTypes;
    (function(ChannelTypes2) {
      ChannelTypes2[ChannelTypes2["GUILD_TEXT"] = 0] = "GUILD_TEXT";
      ChannelTypes2[ChannelTypes2["DM"] = 1] = "DM";
      ChannelTypes2[ChannelTypes2["GUILD_VOICE"] = 2] = "GUILD_VOICE";
      ChannelTypes2[ChannelTypes2["GROUP_DM"] = 3] = "GROUP_DM";
      ChannelTypes2[ChannelTypes2["GUILD_CATEGORY"] = 4] = "GUILD_CATEGORY";
      ChannelTypes2[ChannelTypes2["GUILD_ANNOUNCEMENT"] = 5] = "GUILD_ANNOUNCEMENT";
      ChannelTypes2[ChannelTypes2["GUILD_STORE"] = 6] = "GUILD_STORE";
      ChannelTypes2[ChannelTypes2["ANNOUNCEMENT_THREAD"] = 10] = "ANNOUNCEMENT_THREAD";
      ChannelTypes2[ChannelTypes2["PUBLIC_THREAD"] = 11] = "PUBLIC_THREAD";
      ChannelTypes2[ChannelTypes2["PRIVATE_THREAD"] = 12] = "PRIVATE_THREAD";
      ChannelTypes2[ChannelTypes2["GUILD_STAGE_VOICE"] = 13] = "GUILD_STAGE_VOICE";
      ChannelTypes2[ChannelTypes2["GUILD_DIRECTORY"] = 14] = "GUILD_DIRECTORY";
      ChannelTypes2[ChannelTypes2["GUILD_FORUM"] = 15] = "GUILD_FORUM";
      ChannelTypes2[ChannelTypes2["GUILD_MEDIA"] = 16] = "GUILD_MEDIA";
    })(ChannelTypes || (exports2.ChannelTypes = ChannelTypes = {}));
    var TextStyleTypes;
    (function(TextStyleTypes2) {
      TextStyleTypes2[TextStyleTypes2["SHORT"] = 1] = "SHORT";
      TextStyleTypes2[TextStyleTypes2["PARAGRAPH"] = 2] = "PARAGRAPH";
    })(TextStyleTypes || (exports2.TextStyleTypes = TextStyleTypes = {}));
    var SeparatorSpacingTypes;
    (function(SeparatorSpacingTypes2) {
      SeparatorSpacingTypes2[SeparatorSpacingTypes2["SMALL"] = 1] = "SMALL";
      SeparatorSpacingTypes2[SeparatorSpacingTypes2["LARGE"] = 2] = "LARGE";
    })(SeparatorSpacingTypes || (exports2.SeparatorSpacingTypes = SeparatorSpacingTypes = {}));
  }
});

// node_modules/discord-interactions/dist/index.js
var require_dist = __commonJS({
  "node_modules/discord-interactions/dist/index.js"(exports2) {
    "use strict";
    var __createBinding = exports2 && exports2.__createBinding || (Object.create ? (function(o, m, k, k2) {
      if (k2 === void 0) k2 = k;
      var desc = Object.getOwnPropertyDescriptor(m, k);
      if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
        desc = { enumerable: true, get: function() {
          return m[k];
        } };
      }
      Object.defineProperty(o, k2, desc);
    }) : (function(o, m, k, k2) {
      if (k2 === void 0) k2 = k;
      o[k2] = m[k];
    }));
    var __exportStar = exports2 && exports2.__exportStar || function(m, exports3) {
      for (var p in m) if (p !== "default" && !Object.prototype.hasOwnProperty.call(exports3, p)) __createBinding(exports3, m, p);
    };
    var __awaiter = exports2 && exports2.__awaiter || function(thisArg, _arguments, P, generator) {
      function adopt(value) {
        return value instanceof P ? value : new P(function(resolve) {
          resolve(value);
        });
      }
      return new (P || (P = Promise))(function(resolve, reject) {
        function fulfilled(value) {
          try {
            step(generator.next(value));
          } catch (e) {
            reject(e);
          }
        }
        function rejected(value) {
          try {
            step(generator["throw"](value));
          } catch (e) {
            reject(e);
          }
        }
        function step(result) {
          result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected);
        }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
      });
    };
    Object.defineProperty(exports2, "__esModule", { value: true });
    exports2.verifyWebhookEventMiddleware = exports2.verifyKeyMiddleware = exports2.verifyKey = exports2.InteractionResponseFlags = exports2.InteractionResponseType = exports2.InteractionType = void 0;
    var util_1 = require_util();
    var webhooks_1 = require_webhooks();
    var InteractionType2;
    (function(InteractionType3) {
      InteractionType3[InteractionType3["PING"] = 1] = "PING";
      InteractionType3[InteractionType3["APPLICATION_COMMAND"] = 2] = "APPLICATION_COMMAND";
      InteractionType3[InteractionType3["MESSAGE_COMPONENT"] = 3] = "MESSAGE_COMPONENT";
      InteractionType3[InteractionType3["APPLICATION_COMMAND_AUTOCOMPLETE"] = 4] = "APPLICATION_COMMAND_AUTOCOMPLETE";
      InteractionType3[InteractionType3["MODAL_SUBMIT"] = 5] = "MODAL_SUBMIT";
    })(InteractionType2 || (exports2.InteractionType = InteractionType2 = {}));
    var InteractionResponseType2;
    (function(InteractionResponseType3) {
      InteractionResponseType3[InteractionResponseType3["PONG"] = 1] = "PONG";
      InteractionResponseType3[InteractionResponseType3["CHANNEL_MESSAGE_WITH_SOURCE"] = 4] = "CHANNEL_MESSAGE_WITH_SOURCE";
      InteractionResponseType3[InteractionResponseType3["DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE"] = 5] = "DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE";
      InteractionResponseType3[InteractionResponseType3["DEFERRED_UPDATE_MESSAGE"] = 6] = "DEFERRED_UPDATE_MESSAGE";
      InteractionResponseType3[InteractionResponseType3["UPDATE_MESSAGE"] = 7] = "UPDATE_MESSAGE";
      InteractionResponseType3[InteractionResponseType3["APPLICATION_COMMAND_AUTOCOMPLETE_RESULT"] = 8] = "APPLICATION_COMMAND_AUTOCOMPLETE_RESULT";
      InteractionResponseType3[InteractionResponseType3["MODAL"] = 9] = "MODAL";
      InteractionResponseType3[InteractionResponseType3["PREMIUM_REQUIRED"] = 10] = "PREMIUM_REQUIRED";
      InteractionResponseType3[InteractionResponseType3["LAUNCH_ACTIVITY"] = 12] = "LAUNCH_ACTIVITY";
    })(InteractionResponseType2 || (exports2.InteractionResponseType = InteractionResponseType2 = {}));
    var InteractionResponseFlags;
    (function(InteractionResponseFlags2) {
      InteractionResponseFlags2[InteractionResponseFlags2["EPHEMERAL"] = 64] = "EPHEMERAL";
      InteractionResponseFlags2[InteractionResponseFlags2["IS_COMPONENTS_V2"] = 32768] = "IS_COMPONENTS_V2";
    })(InteractionResponseFlags || (exports2.InteractionResponseFlags = InteractionResponseFlags = {}));
    function verifyKey2(rawBody, signature, timestamp, clientPublicKey) {
      return __awaiter(this, void 0, void 0, function* () {
        try {
          const timestampData = (0, util_1.valueToUint8Array)(timestamp);
          const bodyData = (0, util_1.valueToUint8Array)(rawBody);
          const message = (0, util_1.concatUint8Arrays)(timestampData, bodyData);
          const publicKey = typeof clientPublicKey === "string" ? yield util_1.subtleCrypto.importKey("raw", (0, util_1.valueToUint8Array)(clientPublicKey, "hex"), {
            name: "ed25519",
            namedCurve: "ed25519"
          }, false, ["verify"]) : clientPublicKey;
          const isValid = yield util_1.subtleCrypto.verify({
            name: "ed25519"
          }, publicKey, (0, util_1.valueToUint8Array)(signature, "hex"), message);
          return isValid;
        } catch (_ex) {
          return false;
        }
      });
    }
    exports2.verifyKey = verifyKey2;
    function verifyKeyMiddleware(clientPublicKey) {
      if (!clientPublicKey) {
        throw new Error("You must specify a Discord client public key");
      }
      return (req, res, next) => __awaiter(this, void 0, void 0, function* () {
        const timestamp = req.header("X-Signature-Timestamp") || "";
        const signature = req.header("X-Signature-Ed25519") || "";
        if (!timestamp || !signature) {
          res.statusCode = 401;
          res.end("[discord-interactions] Invalid signature");
          return;
        }
        function onBodyComplete(rawBody) {
          return __awaiter(this, void 0, void 0, function* () {
            const isValid = yield verifyKey2(rawBody, signature, timestamp, clientPublicKey);
            if (!isValid) {
              res.statusCode = 401;
              res.end("[discord-interactions] Invalid signature");
              return;
            }
            const body = JSON.parse(rawBody.toString("utf-8")) || {};
            if (body.type === InteractionType2.PING) {
              res.setHeader("Content-Type", "application/json");
              res.end(JSON.stringify({
                type: InteractionResponseType2.PONG
              }));
              return;
            }
            req.body = body;
            next();
          });
        }
        if (req.body) {
          if (Buffer.isBuffer(req.body)) {
            yield onBodyComplete(req.body);
          } else if (typeof req.body === "string") {
            yield onBodyComplete(Buffer.from(req.body, "utf-8"));
          } else {
            console.warn("[discord-interactions]: req.body was tampered with, probably by some other middleware. We recommend disabling middleware for interaction routes so that req.body is a raw buffer.");
            yield onBodyComplete(Buffer.from(JSON.stringify(req.body), "utf-8"));
          }
        } else {
          const chunks = [];
          req.on("data", (chunk) => {
            chunks.push(chunk);
          });
          req.on("end", () => __awaiter(this, void 0, void 0, function* () {
            const rawBody = Buffer.concat(chunks);
            yield onBodyComplete(rawBody);
          }));
        }
      });
    }
    exports2.verifyKeyMiddleware = verifyKeyMiddleware;
    function verifyWebhookEventMiddleware(clientPublicKey) {
      if (!clientPublicKey) {
        throw new Error("You must specify a Discord client public key");
      }
      return (req, res, next) => __awaiter(this, void 0, void 0, function* () {
        const timestamp = req.header("X-Signature-Timestamp") || "";
        const signature = req.header("X-Signature-Ed25519") || "";
        if (!timestamp || !signature) {
          res.statusCode = 401;
          res.end("[discord-interactions] Invalid signature");
          return;
        }
        function onBodyComplete(rawBody) {
          return __awaiter(this, void 0, void 0, function* () {
            const isValid = yield verifyKey2(rawBody, signature, timestamp, clientPublicKey);
            if (!isValid) {
              res.statusCode = 401;
              res.end("[discord-interactions] Invalid signature");
              return;
            }
            const body = JSON.parse(rawBody.toString("utf-8")) || {};
            if (body.type === webhooks_1.WebhookType.PING) {
              res.statusCode = 204;
              res.end();
              return;
            }
            req.body = body;
            res.statusCode = 204;
            res.end();
            next();
          });
        }
        if (req.body) {
          if (Buffer.isBuffer(req.body)) {
            yield onBodyComplete(req.body);
          } else if (typeof req.body === "string") {
            yield onBodyComplete(Buffer.from(req.body, "utf-8"));
          } else {
            console.warn("[discord-interactions]: req.body was tampered with, probably by some other middleware. We recommend disabling middleware for webhook event routes so that req.body is a raw buffer.");
            yield onBodyComplete(Buffer.from(JSON.stringify(req.body), "utf-8"));
          }
        } else {
          const chunks = [];
          req.on("data", (chunk) => {
            chunks.push(chunk);
          });
          req.on("end", () => __awaiter(this, void 0, void 0, function* () {
            const rawBody = Buffer.concat(chunks);
            yield onBodyComplete(rawBody);
          }));
        }
      });
    }
    exports2.verifyWebhookEventMiddleware = verifyWebhookEventMiddleware;
    __exportStar(require_components(), exports2);
    __exportStar(require_webhooks(), exports2);
  }
});

// src/index.ts
var index_exports = {};
__export(index_exports, {
  handler: () => handler
});
module.exports = __toCommonJS(index_exports);

// node_modules/hono/dist/utils/encode.js
var encodeBase64 = (buf) => {
  let binary = "";
  const bytes = new Uint8Array(buf);
  for (let i = 0, len = bytes.length; i < len; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
};
var decodeBase64 = (str) => {
  const binary = atob(str);
  const bytes = new Uint8Array(new ArrayBuffer(binary.length));
  const half = binary.length / 2;
  for (let i = 0, j = binary.length - 1; i <= half; i++, j--) {
    bytes[i] = binary.charCodeAt(i);
    bytes[j] = binary.charCodeAt(j);
  }
  return bytes;
};

// node_modules/hono/dist/adapter/aws-lambda/handler.js
function sanitizeHeaderValue(value) {
  const hasNonAscii = /[^\x00-\x7F]/.test(value);
  if (!hasNonAscii) {
    return value;
  }
  return encodeURIComponent(value);
}
var getRequestContext = (event) => {
  return event.requestContext;
};
var streamToNodeStream = async (reader, writer) => {
  let readResult = await reader.read();
  while (!readResult.done) {
    writer.write(readResult.value);
    readResult = await reader.read();
  }
  writer.end();
};
var streamHandle = (app2) => {
  return awslambda.streamifyResponse(
    async (event, responseStream, context) => {
      const processor = getProcessor(event);
      try {
        const req = processor.createRequest(event);
        const requestContext = getRequestContext(event);
        const res = await app2.fetch(req, {
          event,
          requestContext,
          context
        });
        const headers = {};
        const cookies = [];
        res.headers.forEach((value, name) => {
          if (name === "set-cookie") {
            cookies.push(value);
          } else {
            headers[name] = value;
          }
        });
        const httpResponseMetadata = {
          statusCode: res.status,
          headers,
          cookies
        };
        responseStream = awslambda.HttpResponseStream.from(responseStream, httpResponseMetadata);
        if (res.body) {
          await streamToNodeStream(res.body.getReader(), responseStream);
        } else {
          responseStream.write("");
        }
      } catch (error) {
        console.error("Error processing request:", error);
        responseStream.write("Internal Server Error");
      } finally {
        responseStream.end();
      }
    }
  );
};
var EventProcessor = class {
  getHeaderValue(headers, key) {
    const value = headers ? Array.isArray(headers[key]) ? headers[key][0] : headers[key] : void 0;
    return value;
  }
  getDomainName(event) {
    if (event.requestContext && "domainName" in event.requestContext) {
      return event.requestContext.domainName;
    }
    const hostFromHeaders = this.getHeaderValue(event.headers, "host");
    if (hostFromHeaders) {
      return hostFromHeaders;
    }
    const multiValueHeaders = "multiValueHeaders" in event ? event.multiValueHeaders : {};
    const hostFromMultiValueHeaders = this.getHeaderValue(multiValueHeaders, "host");
    return hostFromMultiValueHeaders;
  }
  createRequest(event) {
    const queryString = this.getQueryString(event);
    const domainName = this.getDomainName(event);
    const path2 = this.getPath(event);
    const urlPath = `https://${domainName}${path2}`;
    const url = queryString ? `${urlPath}?${queryString}` : urlPath;
    const headers = this.getHeaders(event);
    const method = this.getMethod(event);
    const requestInit = {
      headers,
      method
    };
    if (event.body) {
      requestInit.body = event.isBase64Encoded ? decodeBase64(event.body) : event.body;
    }
    return new Request(url, requestInit);
  }
  async createResult(event, res, options) {
    const contentType = res.headers.get("content-type");
    const isContentTypeBinary = options.isContentTypeBinary ?? defaultIsContentTypeBinary;
    let isBase64Encoded = contentType && isContentTypeBinary(contentType) ? true : false;
    if (!isBase64Encoded) {
      const contentEncoding = res.headers.get("content-encoding");
      isBase64Encoded = isContentEncodingBinary(contentEncoding);
    }
    const body = isBase64Encoded ? encodeBase64(await res.arrayBuffer()) : await res.text();
    const result = {
      body,
      statusCode: res.status,
      isBase64Encoded,
      ..."multiValueHeaders" in event && event.multiValueHeaders ? {
        multiValueHeaders: {}
      } : {
        headers: {}
      }
    };
    this.setCookies(event, res, result);
    if (result.multiValueHeaders) {
      res.headers.forEach((value, key) => {
        result.multiValueHeaders[key] = [value];
      });
    } else {
      res.headers.forEach((value, key) => {
        result.headers[key] = value;
      });
    }
    return result;
  }
  setCookies(_event, res, result) {
    if (res.headers.has("set-cookie")) {
      const cookies = res.headers.getSetCookie ? res.headers.getSetCookie() : Array.from(res.headers.entries()).filter(([k]) => k === "set-cookie").map(([, v]) => v);
      if (Array.isArray(cookies)) {
        this.setCookiesToResult(result, cookies);
        res.headers.delete("set-cookie");
      }
    }
  }
};
var EventV2Processor = class extends EventProcessor {
  getPath(event) {
    return event.rawPath;
  }
  getMethod(event) {
    return event.requestContext.http.method;
  }
  getQueryString(event) {
    return event.rawQueryString;
  }
  getCookies(event, headers) {
    if (Array.isArray(event.cookies)) {
      headers.set("Cookie", event.cookies.join("; "));
    }
  }
  setCookiesToResult(result, cookies) {
    result.cookies = cookies;
  }
  getHeaders(event) {
    const headers = new Headers();
    this.getCookies(event, headers);
    if (event.headers) {
      for (const [k, v] of Object.entries(event.headers)) {
        if (v) {
          headers.set(k, v);
        }
      }
    }
    return headers;
  }
};
var v2Processor = new EventV2Processor();
var EventV1Processor = class extends EventProcessor {
  getPath(event) {
    return event.path;
  }
  getMethod(event) {
    return event.httpMethod;
  }
  getQueryString(event) {
    if (event.multiValueQueryStringParameters) {
      return Object.entries(event.multiValueQueryStringParameters || {}).filter(([, value]) => value).map(
        ([key, values]) => values.map((value) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`).join("&")
      ).join("&");
    } else {
      return Object.entries(event.queryStringParameters || {}).filter(([, value]) => value).map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value || "")}`).join("&");
    }
  }
  getCookies(_event, _headers) {
  }
  getHeaders(event) {
    const headers = new Headers();
    this.getCookies(event, headers);
    if (event.headers) {
      for (const [k, v] of Object.entries(event.headers)) {
        if (v) {
          headers.set(k, sanitizeHeaderValue(v));
        }
      }
    }
    if (event.multiValueHeaders) {
      for (const [k, values] of Object.entries(event.multiValueHeaders)) {
        if (values) {
          const foundK = headers.get(k);
          values.forEach((v) => {
            const sanitizedValue = sanitizeHeaderValue(v);
            return (!foundK || !foundK.includes(sanitizedValue)) && headers.append(k, sanitizedValue);
          });
        }
      }
    }
    return headers;
  }
  setCookiesToResult(result, cookies) {
    result.multiValueHeaders = {
      "set-cookie": cookies
    };
  }
};
var v1Processor = new EventV1Processor();
var ALBProcessor = class extends EventProcessor {
  getHeaders(event) {
    const headers = new Headers();
    if (event.multiValueHeaders) {
      for (const [key, values] of Object.entries(event.multiValueHeaders)) {
        if (values && Array.isArray(values)) {
          const sanitizedValue = sanitizeHeaderValue(values.join("; "));
          headers.set(key, sanitizedValue);
        }
      }
    } else {
      for (const [key, value] of Object.entries(event.headers ?? {})) {
        if (value) {
          headers.set(key, sanitizeHeaderValue(value));
        }
      }
    }
    return headers;
  }
  getPath(event) {
    return event.path;
  }
  getMethod(event) {
    return event.httpMethod;
  }
  getQueryString(event) {
    if (event.multiValueQueryStringParameters) {
      return Object.entries(event.multiValueQueryStringParameters || {}).filter(([, value]) => value).map(([key, value]) => `${key}=${value.join(`&${key}=`)}`).join("&");
    } else {
      return Object.entries(event.queryStringParameters || {}).filter(([, value]) => value).map(([key, value]) => `${key}=${value}`).join("&");
    }
  }
  getCookies(event, headers) {
    let cookie;
    if (event.multiValueHeaders) {
      cookie = event.multiValueHeaders["cookie"]?.join("; ");
    } else {
      cookie = event.headers ? event.headers["cookie"] : void 0;
    }
    if (cookie) {
      headers.append("Cookie", cookie);
    }
  }
  setCookiesToResult(result, cookies) {
    if (result.multiValueHeaders) {
      result.multiValueHeaders["set-cookie"] = cookies;
    } else {
      result.headers["set-cookie"] = cookies.join(", ");
    }
  }
};
var albProcessor = new ALBProcessor();
var LatticeV2Processor = class extends EventProcessor {
  getPath(event) {
    return event.path;
  }
  getMethod(event) {
    return event.method;
  }
  getQueryString() {
    return "";
  }
  getHeaders(event) {
    const headers = new Headers();
    if (event.headers) {
      for (const [k, values] of Object.entries(event.headers)) {
        if (values) {
          const foundK = headers.get(k);
          values.forEach((v) => {
            const sanitizedValue = sanitizeHeaderValue(v);
            return (!foundK || !foundK.includes(sanitizedValue)) && headers.append(k, sanitizedValue);
          });
        }
      }
    }
    return headers;
  }
  getCookies() {
  }
  setCookiesToResult(result, cookies) {
    result.headers = {
      ...result.headers,
      "set-cookie": cookies.join(", ")
    };
  }
};
var latticeV2Processor = new LatticeV2Processor();
var getProcessor = (event) => {
  if (isProxyEventALB(event)) {
    return albProcessor;
  }
  if (isProxyEventV2(event)) {
    return v2Processor;
  }
  if (isLatticeEventV2(event)) {
    return latticeV2Processor;
  }
  return v1Processor;
};
var isProxyEventALB = (event) => {
  if (event.requestContext) {
    return Object.hasOwn(event.requestContext, "elb");
  }
  return false;
};
var isProxyEventV2 = (event) => {
  return Object.hasOwn(event, "rawPath");
};
var isLatticeEventV2 = (event) => {
  if (event.requestContext) {
    return Object.hasOwn(event.requestContext, "serviceArn");
  }
  return false;
};
var defaultIsContentTypeBinary = (contentType) => {
  return !/^text\/(?:plain|html|css|javascript|csv)|(?:\/|\+)(?:json|xml)\s*(?:;|$)/.test(
    contentType
  );
};
var isContentEncodingBinary = (contentEncoding) => {
  if (contentEncoding === null) {
    return false;
  }
  return /^(gzip|deflate|compress|br)/.test(contentEncoding);
};

// src/backends/ecs.ts
var import_client_ecs = require("@aws-sdk/client-ecs");
var import_client_ec2 = require("@aws-sdk/client-ec2");
var REGION = process.env.AWS_REGION ?? "ca-central-1";
var CLUSTER = process.env.ECS_CLUSTER ?? "";
var SIDECAR_TOKEN = process.env.SIDECAR_TOKEN ?? "";
var ecs = new import_client_ecs.ECSClient({ region: REGION });
var ec2 = new import_client_ec2.EC2Client({ region: REGION });
var EcsBackend = class {
  getGames() {
    return JSON.parse(process.env.GAMES ?? "{}");
  }
  async getGameState(config) {
    const c = config;
    try {
      const listRes = await ecs.send(new import_client_ecs.ListTasksCommand({ cluster: CLUSTER, serviceName: c.serviceName }));
      const taskArn = listRes.taskArns?.[0];
      if (!taskArn) return { status: "offline", players: 0, ready: false };
      const descRes = await ecs.send(new import_client_ecs.DescribeTasksCommand({ cluster: CLUSTER, tasks: [taskArn] }));
      const task = descRes.tasks?.[0];
      const eniId = task?.attachments?.[0]?.details?.find((d) => d.name === "networkInterfaceId")?.value;
      if (!eniId) return { status: "starting", players: 0, ready: false };
      const eniRes = await ec2.send(new import_client_ec2.DescribeNetworkInterfacesCommand({ NetworkInterfaceIds: [eniId] }));
      const publicIp = eniRes.NetworkInterfaces?.[0]?.Association?.PublicIp;
      if (!publicIp) return { status: "starting", players: 0, ready: false };
      const sidecar = await getSidecarStatus(publicIp, c.sidecarPort);
      if (!sidecar) return { status: "starting", publicIp, players: 0, ready: false };
      const running = Boolean(sidecar.running);
      const ready = Boolean(sidecar.ready);
      const players = Number(sidecar.players ?? 0);
      return { status: running ? "online" : "starting", publicIp, players, ready };
    } catch {
      return { status: "offline", players: 0, ready: false };
    }
  }
  async getCachedState(config) {
    const c = config;
    const offline = { status: "offline", players: 0, hostname: "", map: "", updatedAt: /* @__PURE__ */ new Date() };
    try {
      const listRes = await ecs.send(new import_client_ecs.ListTasksCommand({ cluster: CLUSTER, serviceName: c.serviceName }));
      const taskArn = listRes.taskArns?.[0];
      if (!taskArn) return offline;
      const descRes = await ecs.send(new import_client_ecs.DescribeTasksCommand({ cluster: CLUSTER, tasks: [taskArn] }));
      const task = descRes.tasks?.[0];
      const eniId = task?.attachments?.[0]?.details?.find((d) => d.name === "networkInterfaceId")?.value;
      if (!eniId) return { ...offline, status: "starting" };
      const eniRes = await ec2.send(new import_client_ec2.DescribeNetworkInterfacesCommand({ NetworkInterfaceIds: [eniId] }));
      const publicIp = eniRes.NetworkInterfaces?.[0]?.Association?.PublicIp;
      if (!publicIp) return { ...offline, status: "starting" };
      const sidecar = await getSidecarStatus(publicIp, c.sidecarPort);
      if (!sidecar) return { ...offline, status: "starting" };
      const running = Boolean(sidecar.running);
      return {
        status: running ? "online" : "starting",
        publicIp,
        players: Number(sidecar.players ?? 0),
        hostname: String(sidecar.hostname ?? ""),
        map: String(sidecar.map ?? ""),
        updatedAt: /* @__PURE__ */ new Date()
      };
    } catch {
      return offline;
    }
  }
  async stopGame(config) {
    const c = config;
    await setDesiredCount(c.serviceName, 0);
    return { status: "offline", players: 0, ready: false };
  }
  async startGame(config, configUrl) {
    const c = config;
    const current = await this.getGameState(config);
    if (current.status === "online" && !configUrl) return current;
    await setDesiredCount(c.serviceName, 1);
    if (configUrl && current.publicIp) {
      await restartWithConfig(current.publicIp, c.sidecarPort, configUrl);
      return { ...current, configUrl };
    }
    return { status: "starting", players: 0, ready: false };
  }
};
async function setDesiredCount(serviceName, count) {
  await ecs.send(new import_client_ecs.UpdateServiceCommand({ cluster: CLUSTER, service: serviceName, desiredCount: count }));
}
async function getSidecarStatus(ip, port) {
  try {
    const res = await fetch(`http://${ip}:${port}/status`, { signal: AbortSignal.timeout(5e3) });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}
async function restartWithConfig(ip, port, configUrl) {
  await fetch(`http://${ip}:${port}/restart`, {
    method: "POST",
    headers: { "Authorization": `Bearer ${SIDECAR_TOKEN}`, "Content-Type": "application/json" },
    body: JSON.stringify({ config_url: configUrl }),
    signal: AbortSignal.timeout(1e4)
  });
}

// src/backends/docker.ts
var import_http = __toESM(require("http"));

// src/game-definitions.ts
var import_fs = require("fs");
var import_path = __toESM(require("path"));
function loadDockerGameDefinitions(repoRoot) {
  const dockerRoot = import_path.default.join(repoRoot, "docker-containers");
  const entries = (0, import_fs.readdirSync)(dockerRoot, { withFileTypes: true }).filter((entry) => entry.isDirectory()).map((entry) => entry.name).sort();
  const definitions = [];
  for (const entry of entries) {
    const gameDir = import_path.default.join(dockerRoot, entry);
    const dockerfilePath = import_path.default.join(gameDir, "Dockerfile");
    const metadataPath = import_path.default.join(gameDir, "game.json");
    try {
      const metadata = JSON.parse((0, import_fs.readFileSync)(metadataPath, "utf8"));
      (0, import_fs.readFileSync)(dockerfilePath, "utf8");
      definitions.push(metadata);
    } catch {
      continue;
    }
  }
  return definitions;
}

// src/logger.ts
function ts() {
  return (/* @__PURE__ */ new Date()).toISOString();
}
var log = {
  info: (msg) => console.log(`${ts()} INFO  ${msg}`),
  warn: (msg) => console.warn(`${ts()} WARN  ${msg}`),
  error: (msg, err) => {
    const detail = err instanceof Error ? ` \u2014 ${err.message}` : err != null ? ` \u2014 ${String(err)}` : "";
    console.error(`${ts()} ERROR ${msg}${detail}`);
  }
};

// src/backends/docker.ts
var SOCKET = process.env.DOCKER_SOCKET ?? "/var/run/docker.sock";
var SIDECAR_TOKEN2 = process.env.SIDECAR_TOKEN ?? "";
var SIDECAR_HOST = process.env.SIDECAR_HOST ?? "localhost";
var DATA_DIR = process.env.DATA_DIR ?? "/data";
var HOST_DATA_DIR = process.env.HOST_DATA_DIR ?? DATA_DIR;
var MAX_POLLS = 20;
var POLL_INTERVAL_MS = 3e3;
var RCON_PASSWORD = process.env.RCON_PASSWORD ?? "";
function dockerRequest(method, path2, body) {
  return new Promise((resolve, reject) => {
    const bodyStr = body ? JSON.stringify(body) : void 0;
    const req = import_http.default.request({
      socketPath: SOCKET,
      path: path2,
      method,
      headers: {
        "Content-Type": "application/json",
        ...bodyStr ? { "Content-Length": Buffer.byteLength(bodyStr) } : {}
      }
    }, (res) => {
      const chunks = [];
      res.on("data", (c) => chunks.push(c));
      res.on("end", () => {
        const text = Buffer.concat(chunks).toString();
        const parsed = (() => {
          try {
            return text ? JSON.parse(text) : null;
          } catch {
            return text;
          }
        })();
        const status = res.statusCode ?? 0;
        if (status < 200 || status >= 300) {
          const message = parsed?.message ?? text;
          reject(new Error(`Docker API ${method} ${path2} \u2192 ${status}: ${message}`));
        } else {
          resolve(parsed);
        }
      });
    });
    req.on("error", reject);
    if (bodyStr) req.write(bodyStr);
    req.end();
  });
}
function pullImage(image) {
  return new Promise((resolve, reject) => {
    const req = import_http.default.request({
      socketPath: SOCKET,
      path: `/images/create?fromImage=${encodeURIComponent(image)}`,
      method: "POST"
    }, (res) => {
      res.on("data", () => {
      });
      res.on("end", () => {
        const status = res.statusCode ?? 0;
        if (status < 200 || status >= 300) {
          reject(new Error(`Docker pull ${image} \u2192 ${status}`));
        } else {
          resolve();
        }
      });
    });
    req.on("error", reject);
    req.end();
  });
}
async function removeContainer(name) {
  await dockerRequest("DELETE", `/containers/${encodeURIComponent(name)}?force=true`);
}
async function ensureContainer(c) {
  const existing = await inspectContainer(c.containerName);
  if (existing) {
    const existingImage = existing.Config?.Image ?? "";
    const expectedBinds = (c.volumes ?? []).map((v) => {
      const [hostPath, ...rest] = v.split(":");
      const absHost = hostPath.startsWith("/") ? hostPath : `${HOST_DATA_DIR}/${hostPath}`;
      return [absHost, ...rest].join(":");
    });
    const existingBinds = existing.HostConfig?.Binds ?? [];
    const bindsMatch = expectedBinds.every((b) => existingBinds.includes(b));
    const imageMatches = existingImage === c.image || existingImage.startsWith(c.image + "@");
    if (imageMatches && bindsMatch) return;
    log.info(`docker: removing stale container ${c.containerName} (image or mounts changed)`);
    await removeContainer(c.containerName);
  }
  log.info(`docker: image pull ${c.image}`);
  await pullImage(c.image);
  log.info(`docker: pulled ${c.image}`);
  const portBindings = {};
  const exposedPorts = {};
  for (const [containerPort, binding] of Object.entries(c.ports ?? {})) {
    const key = containerPort.includes("/") ? containerPort : `${containerPort}/tcp`;
    portBindings[key] = [{ HostIp: binding.hostIp ?? "0.0.0.0", HostPort: binding.hostPort }];
    exposedPorts[key] = {};
  }
  const sidecarKey = `${c.sidecarPort}/tcp`;
  if (!portBindings[sidecarKey]) {
    portBindings[sidecarKey] = [{ HostIp: "127.0.0.1", HostPort: String(c.sidecarPort) }];
    exposedPorts[sidecarKey] = {};
  }
  const binds = (c.volumes ?? []).map((v) => {
    const [hostPath, ...rest] = v.split(":");
    const absHost = hostPath.startsWith("/") ? hostPath : `${HOST_DATA_DIR}/${hostPath}`;
    return [absHost, ...rest].join(":");
  });
  const env = Object.entries(c.environment ?? {}).map(([k, v]) => `${k}=${v}`);
  log.info(`docker: creating container ${c.containerName}`);
  await dockerRequest("POST", `/containers/create?name=${encodeURIComponent(c.containerName)}`, {
    Image: c.image,
    Env: env,
    ExposedPorts: exposedPorts,
    HostConfig: {
      PortBindings: portBindings,
      Binds: binds,
      RestartPolicy: { Name: "no" }
    }
  });
  log.info(`docker: created container ${c.containerName}`);
}
async function inspectContainer(name) {
  try {
    const data = await dockerRequest("GET", `/containers/${encodeURIComponent(name)}/json`);
    return data;
  } catch {
    return null;
  }
}
function getHostPort(inspect, containerPort) {
  const ports = inspect.NetworkSettings?.Ports;
  const key = `${containerPort}/tcp`;
  const binding = ports?.[key]?.[0];
  return binding ? parseInt(binding.HostPort, 10) : null;
}
async function getSidecarStatus2(port) {
  try {
    const res = await fetch(`http://${SIDECAR_HOST}:${port}/status`, { signal: AbortSignal.timeout(5e3) });
    if (!res.ok) return null;
    return res.json();
  } catch {
    return null;
  }
}
async function waitForState(backend2, config, desired) {
  const c = config;
  let state = await backend2.getGameState(config);
  for (let i = 0; i < MAX_POLLS; i++) {
    if (state.status === desired) return state;
    log.info(`docker: waiting for ${c.containerName} to be ${desired} (currently ${state.status}, poll ${i + 1}/${MAX_POLLS})`);
    await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
    state = await backend2.getGameState(config);
  }
  if (state.status !== desired) log.warn(`docker: ${c.containerName} did not reach ${desired} after ${MAX_POLLS} polls (stuck at ${state.status})`);
  return state;
}
async function restartWithConfig2(port, configUrl) {
  await fetch(`http://${SIDECAR_HOST}:${port}/restart`, {
    method: "POST",
    headers: { "Authorization": `Bearer ${SIDECAR_TOKEN2}`, "Content-Type": "application/json" },
    body: JSON.stringify({ config_url: configUrl }),
    signal: AbortSignal.timeout(1e4)
  });
}
var DockerBackend = class {
  getGames() {
    const configs = {};
    for (const definition of loadDockerGameDefinitions(DATA_DIR)) {
      const environment = {
        GAME_CMD: definition.gameCmd,
        GAME_ARGS: definition.gameArgs ?? "",
        GAME_QUIT_CMD: definition.gameQuitCmd ?? "quit",
        GAME_QUIT_TIMEOUT: String(definition.gameQuitTimeout ?? 15),
        GAME_PORT: String(definition.gamePort ?? 26e3),
        TOKEN: SIDECAR_TOKEN2
      };
      if (definition.configPath) environment.CONFIG_PATH = definition.configPath;
      if (definition.dataUrlEnv) {
        const value = process.env[definition.dataUrlEnv];
        if (value) environment.DATA_URL = value;
      }
      if (RCON_PASSWORD) environment.RCON_PASSWORD = RCON_PASSWORD;
      configs[definition.id] = {
        containerName: definition.containerName ?? `insta-game-${definition.id}-1`,
        image: definition.image ?? `ghcr.io/fogo-sh/insta-game:${definition.id}`,
        displayName: definition.displayName,
        connectPort: definition.gamePort,
        clientDownloadUrl: definition.clientDownloadUrl,
        sidecarPort: definition.sidecarPort,
        ports: definition.ports,
        environment,
        volumes: definition.volumes
      };
    }
    return configs;
  }
  async getGameState(config) {
    const c = config;
    try {
      const inspect = await inspectContainer(c.containerName);
      if (!inspect) return { status: "offline", players: 0, ready: false };
      const state = inspect.State;
      if (!state?.Running) return { status: "offline", players: 0, ready: false };
      const hostPort = getHostPort(inspect, c.sidecarPort);
      if (!hostPort) return { status: "starting", players: 0, ready: false };
      const sidecar = await getSidecarStatus2(hostPort);
      if (!sidecar) return { status: "starting", players: 0, ready: false };
      const running = Boolean(sidecar.running);
      const ready = Boolean(sidecar.ready);
      const players = Number(sidecar.players ?? 0);
      return { status: running && ready ? "online" : "starting", publicIp: "localhost", players, ready };
    } catch {
      return { status: "offline", players: 0, ready: false };
    }
  }
  async getCachedState(config) {
    const c = config;
    const offline = { status: "offline", players: 0, hostname: "", map: "", updatedAt: /* @__PURE__ */ new Date() };
    try {
      const inspect = await inspectContainer(c.containerName);
      if (!inspect) return offline;
      const state = inspect.State;
      if (!state?.Running) return offline;
      const hostPort = getHostPort(inspect, c.sidecarPort);
      if (!hostPort) return { ...offline, status: "starting" };
      const sidecar = await getSidecarStatus2(hostPort);
      if (!sidecar) return { ...offline, status: "starting" };
      const running = Boolean(sidecar.running);
      const ready = Boolean(sidecar.ready);
      return {
        status: running && ready ? "online" : "starting",
        players: Number(sidecar.players ?? 0),
        hostname: String(sidecar.hostname ?? ""),
        map: String(sidecar.map ?? ""),
        updatedAt: /* @__PURE__ */ new Date()
      };
    } catch {
      return offline;
    }
  }
  async startGame(config, configUrl) {
    const c = config;
    log.info(`docker: starting container ${c.containerName}`);
    try {
      await ensureContainer(c);
      await dockerRequest("POST", `/containers/${encodeURIComponent(c.containerName)}/start`);
    } catch (err) {
      log.error(`docker: failed to start ${c.containerName}`, err);
      return { status: "offline", players: 0, ready: false };
    }
    let state = await waitForState(this, config, "online");
    if (configUrl && state.status === "online") {
      const inspect = await inspectContainer(c.containerName);
      const hostPort = inspect ? getHostPort(inspect, c.sidecarPort) : null;
      if (hostPort) {
        await restartWithConfig2(hostPort, configUrl);
        state = await waitForState(this, config, "online");
        state.configUrl = configUrl;
      }
    }
    return state;
  }
  async stopGame(config) {
    const c = config;
    log.info(`docker: stopping container ${c.containerName}`);
    try {
      await dockerRequest("POST", `/containers/${encodeURIComponent(c.containerName)}/stop?t=15`);
    } catch (err) {
      log.error(`docker: failed to stop ${c.containerName}`, err);
      return { status: "offline", players: 0, ready: false };
    }
    return waitForState(this, config, "offline");
  }
};

// src/backends/index.ts
function createBackend() {
  const backend2 = process.env.BACKEND ?? "ecs";
  if (backend2 === "docker") return new DockerBackend();
  return new EcsBackend();
}

// src/cache.ts
var POLL_INTERVAL_MS2 = 5e3;
var GameCache = class {
  constructor(backend2) {
    this.backend = backend2;
  }
  backend;
  cache = /* @__PURE__ */ new Map();
  timer = null;
  polling = false;
  lastPolledAt = 0;
  start() {
    if (this.timer) return;
    void this.pollAll();
    this.timer = setInterval(() => {
      void this.pollAll();
    }, POLL_INTERVAL_MS2);
  }
  stop() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }
  get(gameKey) {
    return this.cache.get(gameKey) ?? null;
  }
  set(gameKey, state) {
    this.cache.set(gameKey, state);
  }
  async refreshIfStale(maxAgeMs = POLL_INTERVAL_MS2) {
    if (Date.now() - this.lastPolledAt <= maxAgeMs) return;
    await this.pollAll();
  }
  async pollAll() {
    if (this.polling) return;
    this.polling = true;
    try {
      const games = this.backend.getGames();
      await Promise.all(
        Object.entries(games).map(async ([key, config]) => {
          try {
            const state = await this.backend.getCachedState(config);
            this.cache.set(key, state);
          } catch (err) {
            log.error(`cache: poll failed for ${key}`, err);
            this.cache.set(key, {
              status: "offline",
              players: 0,
              hostname: "",
              map: "",
              updatedAt: /* @__PURE__ */ new Date()
            });
          }
        })
      );
      this.lastPolledAt = Date.now();
    } finally {
      this.polling = false;
    }
  }
};

// node_modules/hono/dist/compose.js
var compose = (middleware, onError, onNotFound) => {
  return (context, next) => {
    let index = -1;
    return dispatch(0);
    async function dispatch(i) {
      if (i <= index) {
        throw new Error("next() called multiple times");
      }
      index = i;
      let res;
      let isError = false;
      let handler2;
      if (middleware[i]) {
        handler2 = middleware[i][0][0];
        context.req.routeIndex = i;
      } else {
        handler2 = i === middleware.length && next || void 0;
      }
      if (handler2) {
        try {
          res = await handler2(context, () => dispatch(i + 1));
        } catch (err) {
          if (err instanceof Error && onError) {
            context.error = err;
            res = await onError(err, context);
            isError = true;
          } else {
            throw err;
          }
        }
      } else {
        if (context.finalized === false && onNotFound) {
          res = await onNotFound(context);
        }
      }
      if (res && (context.finalized === false || isError)) {
        context.res = res;
      }
      return context;
    }
  };
};

// node_modules/hono/dist/request/constants.js
var GET_MATCH_RESULT = /* @__PURE__ */ Symbol();

// node_modules/hono/dist/utils/body.js
var parseBody = async (request, options = /* @__PURE__ */ Object.create(null)) => {
  const { all = false, dot = false } = options;
  const headers = request instanceof HonoRequest ? request.raw.headers : request.headers;
  const contentType = headers.get("Content-Type");
  if (contentType?.startsWith("multipart/form-data") || contentType?.startsWith("application/x-www-form-urlencoded")) {
    return parseFormData(request, { all, dot });
  }
  return {};
};
async function parseFormData(request, options) {
  const formData = await request.formData();
  if (formData) {
    return convertFormDataToBodyData(formData, options);
  }
  return {};
}
function convertFormDataToBodyData(formData, options) {
  const form2 = /* @__PURE__ */ Object.create(null);
  formData.forEach((value, key) => {
    const shouldParseAllValues = options.all || key.endsWith("[]");
    if (!shouldParseAllValues) {
      form2[key] = value;
    } else {
      handleParsingAllValues(form2, key, value);
    }
  });
  if (options.dot) {
    Object.entries(form2).forEach(([key, value]) => {
      const shouldParseDotValues = key.includes(".");
      if (shouldParseDotValues) {
        handleParsingNestedValues(form2, key, value);
        delete form2[key];
      }
    });
  }
  return form2;
}
var handleParsingAllValues = (form2, key, value) => {
  if (form2[key] !== void 0) {
    if (Array.isArray(form2[key])) {
      ;
      form2[key].push(value);
    } else {
      form2[key] = [form2[key], value];
    }
  } else {
    if (!key.endsWith("[]")) {
      form2[key] = value;
    } else {
      form2[key] = [value];
    }
  }
};
var handleParsingNestedValues = (form2, key, value) => {
  if (/(?:^|\.)__proto__\./.test(key)) {
    return;
  }
  let nestedForm = form2;
  const keys = key.split(".");
  keys.forEach((key2, index) => {
    if (index === keys.length - 1) {
      nestedForm[key2] = value;
    } else {
      if (!nestedForm[key2] || typeof nestedForm[key2] !== "object" || Array.isArray(nestedForm[key2]) || nestedForm[key2] instanceof File) {
        nestedForm[key2] = /* @__PURE__ */ Object.create(null);
      }
      nestedForm = nestedForm[key2];
    }
  });
};

// node_modules/hono/dist/utils/url.js
var splitPath = (path2) => {
  const paths = path2.split("/");
  if (paths[0] === "") {
    paths.shift();
  }
  return paths;
};
var splitRoutingPath = (routePath) => {
  const { groups, path: path2 } = extractGroupsFromPath(routePath);
  const paths = splitPath(path2);
  return replaceGroupMarks(paths, groups);
};
var extractGroupsFromPath = (path2) => {
  const groups = [];
  path2 = path2.replace(/\{[^}]+\}/g, (match2, index) => {
    const mark = `@${index}`;
    groups.push([mark, match2]);
    return mark;
  });
  return { groups, path: path2 };
};
var replaceGroupMarks = (paths, groups) => {
  for (let i = groups.length - 1; i >= 0; i--) {
    const [mark] = groups[i];
    for (let j = paths.length - 1; j >= 0; j--) {
      if (paths[j].includes(mark)) {
        paths[j] = paths[j].replace(mark, groups[i][1]);
        break;
      }
    }
  }
  return paths;
};
var patternCache = {};
var getPattern = (label, next) => {
  if (label === "*") {
    return "*";
  }
  const match2 = label.match(/^\:([^\{\}]+)(?:\{(.+)\})?$/);
  if (match2) {
    const cacheKey = `${label}#${next}`;
    if (!patternCache[cacheKey]) {
      if (match2[2]) {
        patternCache[cacheKey] = next && next[0] !== ":" && next[0] !== "*" ? [cacheKey, match2[1], new RegExp(`^${match2[2]}(?=/${next})`)] : [label, match2[1], new RegExp(`^${match2[2]}$`)];
      } else {
        patternCache[cacheKey] = [label, match2[1], true];
      }
    }
    return patternCache[cacheKey];
  }
  return null;
};
var tryDecode = (str, decoder) => {
  try {
    return decoder(str);
  } catch {
    return str.replace(/(?:%[0-9A-Fa-f]{2})+/g, (match2) => {
      try {
        return decoder(match2);
      } catch {
        return match2;
      }
    });
  }
};
var tryDecodeURI = (str) => tryDecode(str, decodeURI);
var getPath = (request) => {
  const url = request.url;
  const start = url.indexOf("/", url.indexOf(":") + 4);
  let i = start;
  for (; i < url.length; i++) {
    const charCode = url.charCodeAt(i);
    if (charCode === 37) {
      const queryIndex = url.indexOf("?", i);
      const hashIndex = url.indexOf("#", i);
      const end = queryIndex === -1 ? hashIndex === -1 ? void 0 : hashIndex : hashIndex === -1 ? queryIndex : Math.min(queryIndex, hashIndex);
      const path2 = url.slice(start, end);
      return tryDecodeURI(path2.includes("%25") ? path2.replace(/%25/g, "%2525") : path2);
    } else if (charCode === 63 || charCode === 35) {
      break;
    }
  }
  return url.slice(start, i);
};
var getPathNoStrict = (request) => {
  const result = getPath(request);
  return result.length > 1 && result.at(-1) === "/" ? result.slice(0, -1) : result;
};
var mergePath = (base, sub, ...rest) => {
  if (rest.length) {
    sub = mergePath(sub, ...rest);
  }
  return `${base?.[0] === "/" ? "" : "/"}${base}${sub === "/" ? "" : `${base?.at(-1) === "/" ? "" : "/"}${sub?.[0] === "/" ? sub.slice(1) : sub}`}`;
};
var checkOptionalParameter = (path2) => {
  if (path2.charCodeAt(path2.length - 1) !== 63 || !path2.includes(":")) {
    return null;
  }
  const segments = path2.split("/");
  const results = [];
  let basePath = "";
  segments.forEach((segment) => {
    if (segment !== "" && !/\:/.test(segment)) {
      basePath += "/" + segment;
    } else if (/\:/.test(segment)) {
      if (/\?/.test(segment)) {
        if (results.length === 0 && basePath === "") {
          results.push("/");
        } else {
          results.push(basePath);
        }
        const optionalSegment = segment.replace("?", "");
        basePath += "/" + optionalSegment;
        results.push(basePath);
      } else {
        basePath += "/" + segment;
      }
    }
  });
  return results.filter((v, i, a) => a.indexOf(v) === i);
};
var _decodeURI = (value) => {
  if (!/[%+]/.test(value)) {
    return value;
  }
  if (value.indexOf("+") !== -1) {
    value = value.replace(/\+/g, " ");
  }
  return value.indexOf("%") !== -1 ? tryDecode(value, decodeURIComponent_) : value;
};
var _getQueryParam = (url, key, multiple) => {
  let encoded;
  if (!multiple && key && !/[%+]/.test(key)) {
    let keyIndex2 = url.indexOf("?", 8);
    if (keyIndex2 === -1) {
      return void 0;
    }
    if (!url.startsWith(key, keyIndex2 + 1)) {
      keyIndex2 = url.indexOf(`&${key}`, keyIndex2 + 1);
    }
    while (keyIndex2 !== -1) {
      const trailingKeyCode = url.charCodeAt(keyIndex2 + key.length + 1);
      if (trailingKeyCode === 61) {
        const valueIndex = keyIndex2 + key.length + 2;
        const endIndex = url.indexOf("&", valueIndex);
        return _decodeURI(url.slice(valueIndex, endIndex === -1 ? void 0 : endIndex));
      } else if (trailingKeyCode == 38 || isNaN(trailingKeyCode)) {
        return "";
      }
      keyIndex2 = url.indexOf(`&${key}`, keyIndex2 + 1);
    }
    encoded = /[%+]/.test(url);
    if (!encoded) {
      return void 0;
    }
  }
  const results = {};
  encoded ??= /[%+]/.test(url);
  let keyIndex = url.indexOf("?", 8);
  while (keyIndex !== -1) {
    const nextKeyIndex = url.indexOf("&", keyIndex + 1);
    let valueIndex = url.indexOf("=", keyIndex);
    if (valueIndex > nextKeyIndex && nextKeyIndex !== -1) {
      valueIndex = -1;
    }
    let name = url.slice(
      keyIndex + 1,
      valueIndex === -1 ? nextKeyIndex === -1 ? void 0 : nextKeyIndex : valueIndex
    );
    if (encoded) {
      name = _decodeURI(name);
    }
    keyIndex = nextKeyIndex;
    if (name === "") {
      continue;
    }
    let value;
    if (valueIndex === -1) {
      value = "";
    } else {
      value = url.slice(valueIndex + 1, nextKeyIndex === -1 ? void 0 : nextKeyIndex);
      if (encoded) {
        value = _decodeURI(value);
      }
    }
    if (multiple) {
      if (!(results[name] && Array.isArray(results[name]))) {
        results[name] = [];
      }
      ;
      results[name].push(value);
    } else {
      results[name] ??= value;
    }
  }
  return key ? results[key] : results;
};
var getQueryParam = _getQueryParam;
var getQueryParams = (url, key) => {
  return _getQueryParam(url, key, true);
};
var decodeURIComponent_ = decodeURIComponent;

// node_modules/hono/dist/request.js
var tryDecodeURIComponent = (str) => tryDecode(str, decodeURIComponent_);
var HonoRequest = class {
  /**
   * `.raw` can get the raw Request object.
   *
   * @see {@link https://hono.dev/docs/api/request#raw}
   *
   * @example
   * ```ts
   * // For Cloudflare Workers
   * app.post('/', async (c) => {
   *   const metadata = c.req.raw.cf?.hostMetadata?
   *   ...
   * })
   * ```
   */
  raw;
  #validatedData;
  // Short name of validatedData
  #matchResult;
  routeIndex = 0;
  /**
   * `.path` can get the pathname of the request.
   *
   * @see {@link https://hono.dev/docs/api/request#path}
   *
   * @example
   * ```ts
   * app.get('/about/me', (c) => {
   *   const pathname = c.req.path // `/about/me`
   * })
   * ```
   */
  path;
  bodyCache = {};
  constructor(request, path2 = "/", matchResult = [[]]) {
    this.raw = request;
    this.path = path2;
    this.#matchResult = matchResult;
    this.#validatedData = {};
  }
  param(key) {
    return key ? this.#getDecodedParam(key) : this.#getAllDecodedParams();
  }
  #getDecodedParam(key) {
    const paramKey = this.#matchResult[0][this.routeIndex][1][key];
    const param = this.#getParamValue(paramKey);
    return param && /\%/.test(param) ? tryDecodeURIComponent(param) : param;
  }
  #getAllDecodedParams() {
    const decoded = {};
    const keys = Object.keys(this.#matchResult[0][this.routeIndex][1]);
    for (const key of keys) {
      const value = this.#getParamValue(this.#matchResult[0][this.routeIndex][1][key]);
      if (value !== void 0) {
        decoded[key] = /\%/.test(value) ? tryDecodeURIComponent(value) : value;
      }
    }
    return decoded;
  }
  #getParamValue(paramKey) {
    return this.#matchResult[1] ? this.#matchResult[1][paramKey] : paramKey;
  }
  query(key) {
    return getQueryParam(this.url, key);
  }
  queries(key) {
    return getQueryParams(this.url, key);
  }
  header(name) {
    if (name) {
      return this.raw.headers.get(name) ?? void 0;
    }
    const headerData = {};
    this.raw.headers.forEach((value, key) => {
      headerData[key] = value;
    });
    return headerData;
  }
  async parseBody(options) {
    return parseBody(this, options);
  }
  #cachedBody = (key) => {
    const { bodyCache, raw: raw2 } = this;
    const cachedBody = bodyCache[key];
    if (cachedBody) {
      return cachedBody;
    }
    const anyCachedKey = Object.keys(bodyCache)[0];
    if (anyCachedKey) {
      return bodyCache[anyCachedKey].then((body) => {
        if (anyCachedKey === "json") {
          body = JSON.stringify(body);
        }
        return new Response(body)[key]();
      });
    }
    return bodyCache[key] = raw2[key]();
  };
  /**
   * `.json()` can parse Request body of type `application/json`
   *
   * @see {@link https://hono.dev/docs/api/request#json}
   *
   * @example
   * ```ts
   * app.post('/entry', async (c) => {
   *   const body = await c.req.json()
   * })
   * ```
   */
  json() {
    return this.#cachedBody("text").then((text) => JSON.parse(text));
  }
  /**
   * `.text()` can parse Request body of type `text/plain`
   *
   * @see {@link https://hono.dev/docs/api/request#text}
   *
   * @example
   * ```ts
   * app.post('/entry', async (c) => {
   *   const body = await c.req.text()
   * })
   * ```
   */
  text() {
    return this.#cachedBody("text");
  }
  /**
   * `.arrayBuffer()` parse Request body as an `ArrayBuffer`
   *
   * @see {@link https://hono.dev/docs/api/request#arraybuffer}
   *
   * @example
   * ```ts
   * app.post('/entry', async (c) => {
   *   const body = await c.req.arrayBuffer()
   * })
   * ```
   */
  arrayBuffer() {
    return this.#cachedBody("arrayBuffer");
  }
  /**
   * Parses the request body as a `Blob`.
   * @example
   * ```ts
   * app.post('/entry', async (c) => {
   *   const body = await c.req.blob();
   * });
   * ```
   * @see https://hono.dev/docs/api/request#blob
   */
  blob() {
    return this.#cachedBody("blob");
  }
  /**
   * Parses the request body as `FormData`.
   * @example
   * ```ts
   * app.post('/entry', async (c) => {
   *   const body = await c.req.formData();
   * });
   * ```
   * @see https://hono.dev/docs/api/request#formdata
   */
  formData() {
    return this.#cachedBody("formData");
  }
  /**
   * Adds validated data to the request.
   *
   * @param target - The target of the validation.
   * @param data - The validated data to add.
   */
  addValidatedData(target, data) {
    this.#validatedData[target] = data;
  }
  valid(target) {
    return this.#validatedData[target];
  }
  /**
   * `.url()` can get the request url strings.
   *
   * @see {@link https://hono.dev/docs/api/request#url}
   *
   * @example
   * ```ts
   * app.get('/about/me', (c) => {
   *   const url = c.req.url // `http://localhost:8787/about/me`
   *   ...
   * })
   * ```
   */
  get url() {
    return this.raw.url;
  }
  /**
   * `.method()` can get the method name of the request.
   *
   * @see {@link https://hono.dev/docs/api/request#method}
   *
   * @example
   * ```ts
   * app.get('/about/me', (c) => {
   *   const method = c.req.method // `GET`
   * })
   * ```
   */
  get method() {
    return this.raw.method;
  }
  get [GET_MATCH_RESULT]() {
    return this.#matchResult;
  }
  /**
   * `.matchedRoutes()` can return a matched route in the handler
   *
   * @deprecated
   *
   * Use matchedRoutes helper defined in "hono/route" instead.
   *
   * @see {@link https://hono.dev/docs/api/request#matchedroutes}
   *
   * @example
   * ```ts
   * app.use('*', async function logger(c, next) {
   *   await next()
   *   c.req.matchedRoutes.forEach(({ handler, method, path }, i) => {
   *     const name = handler.name || (handler.length < 2 ? '[handler]' : '[middleware]')
   *     console.log(
   *       method,
   *       ' ',
   *       path,
   *       ' '.repeat(Math.max(10 - path.length, 0)),
   *       name,
   *       i === c.req.routeIndex ? '<- respond from here' : ''
   *     )
   *   })
   * })
   * ```
   */
  get matchedRoutes() {
    return this.#matchResult[0].map(([[, route]]) => route);
  }
  /**
   * `routePath()` can retrieve the path registered within the handler
   *
   * @deprecated
   *
   * Use routePath helper defined in "hono/route" instead.
   *
   * @see {@link https://hono.dev/docs/api/request#routepath}
   *
   * @example
   * ```ts
   * app.get('/posts/:id', (c) => {
   *   return c.json({ path: c.req.routePath })
   * })
   * ```
   */
  get routePath() {
    return this.#matchResult[0].map(([[, route]]) => route)[this.routeIndex].path;
  }
};

// node_modules/hono/dist/utils/html.js
var HtmlEscapedCallbackPhase = {
  Stringify: 1,
  BeforeStream: 2,
  Stream: 3
};
var raw = (value, callbacks) => {
  const escapedString = new String(value);
  escapedString.isEscaped = true;
  escapedString.callbacks = callbacks;
  return escapedString;
};
var escapeRe = /[&<>'"]/;
var stringBufferToString = async (buffer, callbacks) => {
  let str = "";
  callbacks ||= [];
  const resolvedBuffer = await Promise.all(buffer);
  for (let i = resolvedBuffer.length - 1; ; i--) {
    str += resolvedBuffer[i];
    i--;
    if (i < 0) {
      break;
    }
    let r = resolvedBuffer[i];
    if (typeof r === "object") {
      callbacks.push(...r.callbacks || []);
    }
    const isEscaped = r.isEscaped;
    r = await (typeof r === "object" ? r.toString() : r);
    if (typeof r === "object") {
      callbacks.push(...r.callbacks || []);
    }
    if (r.isEscaped ?? isEscaped) {
      str += r;
    } else {
      const buf = [str];
      escapeToBuffer(r, buf);
      str = buf[0];
    }
  }
  return raw(str, callbacks);
};
var escapeToBuffer = (str, buffer) => {
  const match2 = str.search(escapeRe);
  if (match2 === -1) {
    buffer[0] += str;
    return;
  }
  let escape;
  let index;
  let lastIndex = 0;
  for (index = match2; index < str.length; index++) {
    switch (str.charCodeAt(index)) {
      case 34:
        escape = "&quot;";
        break;
      case 39:
        escape = "&#39;";
        break;
      case 38:
        escape = "&amp;";
        break;
      case 60:
        escape = "&lt;";
        break;
      case 62:
        escape = "&gt;";
        break;
      default:
        continue;
    }
    buffer[0] += str.substring(lastIndex, index) + escape;
    lastIndex = index + 1;
  }
  buffer[0] += str.substring(lastIndex, index);
};
var resolveCallbackSync = (str) => {
  const callbacks = str.callbacks;
  if (!callbacks?.length) {
    return str;
  }
  const buffer = [str];
  const context = {};
  callbacks.forEach((c) => c({ phase: HtmlEscapedCallbackPhase.Stringify, buffer, context }));
  return buffer[0];
};
var resolveCallback = async (str, phase, preserveCallbacks, context, buffer) => {
  if (typeof str === "object" && !(str instanceof String)) {
    if (!(str instanceof Promise)) {
      str = str.toString();
    }
    if (str instanceof Promise) {
      str = await str;
    }
  }
  const callbacks = str.callbacks;
  if (!callbacks?.length) {
    return Promise.resolve(str);
  }
  if (buffer) {
    buffer[0] += str;
  } else {
    buffer = [str];
  }
  const resStr = Promise.all(callbacks.map((c) => c({ phase, buffer, context }))).then(
    (res) => Promise.all(
      res.filter(Boolean).map((str2) => resolveCallback(str2, phase, false, context, buffer))
    ).then(() => buffer[0])
  );
  if (preserveCallbacks) {
    return raw(await resStr, callbacks);
  } else {
    return resStr;
  }
};

// node_modules/hono/dist/context.js
var TEXT_PLAIN = "text/plain; charset=UTF-8";
var setDefaultContentType = (contentType, headers) => {
  return {
    "Content-Type": contentType,
    ...headers
  };
};
var createResponseInstance = (body, init) => new Response(body, init);
var Context = class {
  #rawRequest;
  #req;
  /**
   * `.env` can get bindings (environment variables, secrets, KV namespaces, D1 database, R2 bucket etc.) in Cloudflare Workers.
   *
   * @see {@link https://hono.dev/docs/api/context#env}
   *
   * @example
   * ```ts
   * // Environment object for Cloudflare Workers
   * app.get('*', async c => {
   *   const counter = c.env.COUNTER
   * })
   * ```
   */
  env = {};
  #var;
  finalized = false;
  /**
   * `.error` can get the error object from the middleware if the Handler throws an error.
   *
   * @see {@link https://hono.dev/docs/api/context#error}
   *
   * @example
   * ```ts
   * app.use('*', async (c, next) => {
   *   await next()
   *   if (c.error) {
   *     // do something...
   *   }
   * })
   * ```
   */
  error;
  #status;
  #executionCtx;
  #res;
  #layout;
  #renderer;
  #notFoundHandler;
  #preparedHeaders;
  #matchResult;
  #path;
  /**
   * Creates an instance of the Context class.
   *
   * @param req - The Request object.
   * @param options - Optional configuration options for the context.
   */
  constructor(req, options) {
    this.#rawRequest = req;
    if (options) {
      this.#executionCtx = options.executionCtx;
      this.env = options.env;
      this.#notFoundHandler = options.notFoundHandler;
      this.#path = options.path;
      this.#matchResult = options.matchResult;
    }
  }
  /**
   * `.req` is the instance of {@link HonoRequest}.
   */
  get req() {
    this.#req ??= new HonoRequest(this.#rawRequest, this.#path, this.#matchResult);
    return this.#req;
  }
  /**
   * @see {@link https://hono.dev/docs/api/context#event}
   * The FetchEvent associated with the current request.
   *
   * @throws Will throw an error if the context does not have a FetchEvent.
   */
  get event() {
    if (this.#executionCtx && "respondWith" in this.#executionCtx) {
      return this.#executionCtx;
    } else {
      throw Error("This context has no FetchEvent");
    }
  }
  /**
   * @see {@link https://hono.dev/docs/api/context#executionctx}
   * The ExecutionContext associated with the current request.
   *
   * @throws Will throw an error if the context does not have an ExecutionContext.
   */
  get executionCtx() {
    if (this.#executionCtx) {
      return this.#executionCtx;
    } else {
      throw Error("This context has no ExecutionContext");
    }
  }
  /**
   * @see {@link https://hono.dev/docs/api/context#res}
   * The Response object for the current request.
   */
  get res() {
    return this.#res ||= createResponseInstance(null, {
      headers: this.#preparedHeaders ??= new Headers()
    });
  }
  /**
   * Sets the Response object for the current request.
   *
   * @param _res - The Response object to set.
   */
  set res(_res) {
    if (this.#res && _res) {
      _res = createResponseInstance(_res.body, _res);
      for (const [k, v] of this.#res.headers.entries()) {
        if (k === "content-type") {
          continue;
        }
        if (k === "set-cookie") {
          const cookies = this.#res.headers.getSetCookie();
          _res.headers.delete("set-cookie");
          for (const cookie of cookies) {
            _res.headers.append("set-cookie", cookie);
          }
        } else {
          _res.headers.set(k, v);
        }
      }
    }
    this.#res = _res;
    this.finalized = true;
  }
  /**
   * `.render()` can create a response within a layout.
   *
   * @see {@link https://hono.dev/docs/api/context#render-setrenderer}
   *
   * @example
   * ```ts
   * app.get('/', (c) => {
   *   return c.render('Hello!')
   * })
   * ```
   */
  render = (...args) => {
    this.#renderer ??= (content) => this.html(content);
    return this.#renderer(...args);
  };
  /**
   * Sets the layout for the response.
   *
   * @param layout - The layout to set.
   * @returns The layout function.
   */
  setLayout = (layout) => this.#layout = layout;
  /**
   * Gets the current layout for the response.
   *
   * @returns The current layout function.
   */
  getLayout = () => this.#layout;
  /**
   * `.setRenderer()` can set the layout in the custom middleware.
   *
   * @see {@link https://hono.dev/docs/api/context#render-setrenderer}
   *
   * @example
   * ```tsx
   * app.use('*', async (c, next) => {
   *   c.setRenderer((content) => {
   *     return c.html(
   *       <html>
   *         <body>
   *           <p>{content}</p>
   *         </body>
   *       </html>
   *     )
   *   })
   *   await next()
   * })
   * ```
   */
  setRenderer = (renderer) => {
    this.#renderer = renderer;
  };
  /**
   * `.header()` can set headers.
   *
   * @see {@link https://hono.dev/docs/api/context#header}
   *
   * @example
   * ```ts
   * app.get('/welcome', (c) => {
   *   // Set headers
   *   c.header('X-Message', 'Hello!')
   *   c.header('Content-Type', 'text/plain')
   *
   *   return c.body('Thank you for coming')
   * })
   * ```
   */
  header = (name, value, options) => {
    if (this.finalized) {
      this.#res = createResponseInstance(this.#res.body, this.#res);
    }
    const headers = this.#res ? this.#res.headers : this.#preparedHeaders ??= new Headers();
    if (value === void 0) {
      headers.delete(name);
    } else if (options?.append) {
      headers.append(name, value);
    } else {
      headers.set(name, value);
    }
  };
  status = (status) => {
    this.#status = status;
  };
  /**
   * `.set()` can set the value specified by the key.
   *
   * @see {@link https://hono.dev/docs/api/context#set-get}
   *
   * @example
   * ```ts
   * app.use('*', async (c, next) => {
   *   c.set('message', 'Hono is hot!!')
   *   await next()
   * })
   * ```
   */
  set = (key, value) => {
    this.#var ??= /* @__PURE__ */ new Map();
    this.#var.set(key, value);
  };
  /**
   * `.get()` can use the value specified by the key.
   *
   * @see {@link https://hono.dev/docs/api/context#set-get}
   *
   * @example
   * ```ts
   * app.get('/', (c) => {
   *   const message = c.get('message')
   *   return c.text(`The message is "${message}"`)
   * })
   * ```
   */
  get = (key) => {
    return this.#var ? this.#var.get(key) : void 0;
  };
  /**
   * `.var` can access the value of a variable.
   *
   * @see {@link https://hono.dev/docs/api/context#var}
   *
   * @example
   * ```ts
   * const result = c.var.client.oneMethod()
   * ```
   */
  // c.var.propName is a read-only
  get var() {
    if (!this.#var) {
      return {};
    }
    return Object.fromEntries(this.#var);
  }
  #newResponse(data, arg, headers) {
    const responseHeaders = this.#res ? new Headers(this.#res.headers) : this.#preparedHeaders ?? new Headers();
    if (typeof arg === "object" && "headers" in arg) {
      const argHeaders = arg.headers instanceof Headers ? arg.headers : new Headers(arg.headers);
      for (const [key, value] of argHeaders) {
        if (key.toLowerCase() === "set-cookie") {
          responseHeaders.append(key, value);
        } else {
          responseHeaders.set(key, value);
        }
      }
    }
    if (headers) {
      for (const [k, v] of Object.entries(headers)) {
        if (typeof v === "string") {
          responseHeaders.set(k, v);
        } else {
          responseHeaders.delete(k);
          for (const v2 of v) {
            responseHeaders.append(k, v2);
          }
        }
      }
    }
    const status = typeof arg === "number" ? arg : arg?.status ?? this.#status;
    return createResponseInstance(data, { status, headers: responseHeaders });
  }
  newResponse = (...args) => this.#newResponse(...args);
  /**
   * `.body()` can return the HTTP response.
   * You can set headers with `.header()` and set HTTP status code with `.status`.
   * This can also be set in `.text()`, `.json()` and so on.
   *
   * @see {@link https://hono.dev/docs/api/context#body}
   *
   * @example
   * ```ts
   * app.get('/welcome', (c) => {
   *   // Set headers
   *   c.header('X-Message', 'Hello!')
   *   c.header('Content-Type', 'text/plain')
   *   // Set HTTP status code
   *   c.status(201)
   *
   *   // Return the response body
   *   return c.body('Thank you for coming')
   * })
   * ```
   */
  body = (data, arg, headers) => this.#newResponse(data, arg, headers);
  /**
   * `.text()` can render text as `Content-Type:text/plain`.
   *
   * @see {@link https://hono.dev/docs/api/context#text}
   *
   * @example
   * ```ts
   * app.get('/say', (c) => {
   *   return c.text('Hello!')
   * })
   * ```
   */
  text = (text, arg, headers) => {
    return !this.#preparedHeaders && !this.#status && !arg && !headers && !this.finalized ? new Response(text) : this.#newResponse(
      text,
      arg,
      setDefaultContentType(TEXT_PLAIN, headers)
    );
  };
  /**
   * `.json()` can render JSON as `Content-Type:application/json`.
   *
   * @see {@link https://hono.dev/docs/api/context#json}
   *
   * @example
   * ```ts
   * app.get('/api', (c) => {
   *   return c.json({ message: 'Hello!' })
   * })
   * ```
   */
  json = (object, arg, headers) => {
    return this.#newResponse(
      JSON.stringify(object),
      arg,
      setDefaultContentType("application/json", headers)
    );
  };
  html = (html2, arg, headers) => {
    const res = (html22) => this.#newResponse(html22, arg, setDefaultContentType("text/html; charset=UTF-8", headers));
    return typeof html2 === "object" ? resolveCallback(html2, HtmlEscapedCallbackPhase.Stringify, false, {}).then(res) : res(html2);
  };
  /**
   * `.redirect()` can Redirect, default status code is 302.
   *
   * @see {@link https://hono.dev/docs/api/context#redirect}
   *
   * @example
   * ```ts
   * app.get('/redirect', (c) => {
   *   return c.redirect('/')
   * })
   * app.get('/redirect-permanently', (c) => {
   *   return c.redirect('/', 301)
   * })
   * ```
   */
  redirect = (location, status) => {
    const locationString = String(location);
    this.header(
      "Location",
      // Multibyes should be encoded
      // eslint-disable-next-line no-control-regex
      !/[^\x00-\xFF]/.test(locationString) ? locationString : encodeURI(locationString)
    );
    return this.newResponse(null, status ?? 302);
  };
  /**
   * `.notFound()` can return the Not Found Response.
   *
   * @see {@link https://hono.dev/docs/api/context#notfound}
   *
   * @example
   * ```ts
   * app.get('/notfound', (c) => {
   *   return c.notFound()
   * })
   * ```
   */
  notFound = () => {
    this.#notFoundHandler ??= () => createResponseInstance();
    return this.#notFoundHandler(this);
  };
};

// node_modules/hono/dist/router.js
var METHOD_NAME_ALL = "ALL";
var METHOD_NAME_ALL_LOWERCASE = "all";
var METHODS = ["get", "post", "put", "delete", "options", "patch"];
var MESSAGE_MATCHER_IS_ALREADY_BUILT = "Can not add a route since the matcher is already built.";
var UnsupportedPathError = class extends Error {
};

// node_modules/hono/dist/utils/constants.js
var COMPOSED_HANDLER = "__COMPOSED_HANDLER";

// node_modules/hono/dist/hono-base.js
var notFoundHandler = (c) => {
  return c.text("404 Not Found", 404);
};
var errorHandler = (err, c) => {
  if ("getResponse" in err) {
    const res = err.getResponse();
    return c.newResponse(res.body, res);
  }
  console.error(err);
  return c.text("Internal Server Error", 500);
};
var Hono = class _Hono {
  get;
  post;
  put;
  delete;
  options;
  patch;
  all;
  on;
  use;
  /*
    This class is like an abstract class and does not have a router.
    To use it, inherit the class and implement router in the constructor.
  */
  router;
  getPath;
  // Cannot use `#` because it requires visibility at JavaScript runtime.
  _basePath = "/";
  #path = "/";
  routes = [];
  constructor(options = {}) {
    const allMethods = [...METHODS, METHOD_NAME_ALL_LOWERCASE];
    allMethods.forEach((method) => {
      this[method] = (args1, ...args) => {
        if (typeof args1 === "string") {
          this.#path = args1;
        } else {
          this.#addRoute(method, this.#path, args1);
        }
        args.forEach((handler2) => {
          this.#addRoute(method, this.#path, handler2);
        });
        return this;
      };
    });
    this.on = (method, path2, ...handlers) => {
      for (const p of [path2].flat()) {
        this.#path = p;
        for (const m of [method].flat()) {
          handlers.map((handler2) => {
            this.#addRoute(m.toUpperCase(), this.#path, handler2);
          });
        }
      }
      return this;
    };
    this.use = (arg1, ...handlers) => {
      if (typeof arg1 === "string") {
        this.#path = arg1;
      } else {
        this.#path = "*";
        handlers.unshift(arg1);
      }
      handlers.forEach((handler2) => {
        this.#addRoute(METHOD_NAME_ALL, this.#path, handler2);
      });
      return this;
    };
    const { strict, ...optionsWithoutStrict } = options;
    Object.assign(this, optionsWithoutStrict);
    this.getPath = strict ?? true ? options.getPath ?? getPath : getPathNoStrict;
  }
  #clone() {
    const clone = new _Hono({
      router: this.router,
      getPath: this.getPath
    });
    clone.errorHandler = this.errorHandler;
    clone.#notFoundHandler = this.#notFoundHandler;
    clone.routes = this.routes;
    return clone;
  }
  #notFoundHandler = notFoundHandler;
  // Cannot use `#` because it requires visibility at JavaScript runtime.
  errorHandler = errorHandler;
  /**
   * `.route()` allows grouping other Hono instance in routes.
   *
   * @see {@link https://hono.dev/docs/api/routing#grouping}
   *
   * @param {string} path - base Path
   * @param {Hono} app - other Hono instance
   * @returns {Hono} routed Hono instance
   *
   * @example
   * ```ts
   * const app = new Hono()
   * const app2 = new Hono()
   *
   * app2.get("/user", (c) => c.text("user"))
   * app.route("/api", app2) // GET /api/user
   * ```
   */
  route(path2, app2) {
    const subApp = this.basePath(path2);
    app2.routes.map((r) => {
      let handler2;
      if (app2.errorHandler === errorHandler) {
        handler2 = r.handler;
      } else {
        handler2 = async (c, next) => (await compose([], app2.errorHandler)(c, () => r.handler(c, next))).res;
        handler2[COMPOSED_HANDLER] = r.handler;
      }
      subApp.#addRoute(r.method, r.path, handler2);
    });
    return this;
  }
  /**
   * `.basePath()` allows base paths to be specified.
   *
   * @see {@link https://hono.dev/docs/api/routing#base-path}
   *
   * @param {string} path - base Path
   * @returns {Hono} changed Hono instance
   *
   * @example
   * ```ts
   * const api = new Hono().basePath('/api')
   * ```
   */
  basePath(path2) {
    const subApp = this.#clone();
    subApp._basePath = mergePath(this._basePath, path2);
    return subApp;
  }
  /**
   * `.onError()` handles an error and returns a customized Response.
   *
   * @see {@link https://hono.dev/docs/api/hono#error-handling}
   *
   * @param {ErrorHandler} handler - request Handler for error
   * @returns {Hono} changed Hono instance
   *
   * @example
   * ```ts
   * app.onError((err, c) => {
   *   console.error(`${err}`)
   *   return c.text('Custom Error Message', 500)
   * })
   * ```
   */
  onError = (handler2) => {
    this.errorHandler = handler2;
    return this;
  };
  /**
   * `.notFound()` allows you to customize a Not Found Response.
   *
   * @see {@link https://hono.dev/docs/api/hono#not-found}
   *
   * @param {NotFoundHandler} handler - request handler for not-found
   * @returns {Hono} changed Hono instance
   *
   * @example
   * ```ts
   * app.notFound((c) => {
   *   return c.text('Custom 404 Message', 404)
   * })
   * ```
   */
  notFound = (handler2) => {
    this.#notFoundHandler = handler2;
    return this;
  };
  /**
   * `.mount()` allows you to mount applications built with other frameworks into your Hono application.
   *
   * @see {@link https://hono.dev/docs/api/hono#mount}
   *
   * @param {string} path - base Path
   * @param {Function} applicationHandler - other Request Handler
   * @param {MountOptions} [options] - options of `.mount()`
   * @returns {Hono} mounted Hono instance
   *
   * @example
   * ```ts
   * import { Router as IttyRouter } from 'itty-router'
   * import { Hono } from 'hono'
   * // Create itty-router application
   * const ittyRouter = IttyRouter()
   * // GET /itty-router/hello
   * ittyRouter.get('/hello', () => new Response('Hello from itty-router'))
   *
   * const app = new Hono()
   * app.mount('/itty-router', ittyRouter.handle)
   * ```
   *
   * @example
   * ```ts
   * const app = new Hono()
   * // Send the request to another application without modification.
   * app.mount('/app', anotherApp, {
   *   replaceRequest: (req) => req,
   * })
   * ```
   */
  mount(path2, applicationHandler, options) {
    let replaceRequest;
    let optionHandler;
    if (options) {
      if (typeof options === "function") {
        optionHandler = options;
      } else {
        optionHandler = options.optionHandler;
        if (options.replaceRequest === false) {
          replaceRequest = (request) => request;
        } else {
          replaceRequest = options.replaceRequest;
        }
      }
    }
    const getOptions = optionHandler ? (c) => {
      const options2 = optionHandler(c);
      return Array.isArray(options2) ? options2 : [options2];
    } : (c) => {
      let executionContext = void 0;
      try {
        executionContext = c.executionCtx;
      } catch {
      }
      return [c.env, executionContext];
    };
    replaceRequest ||= (() => {
      const mergedPath = mergePath(this._basePath, path2);
      const pathPrefixLength = mergedPath === "/" ? 0 : mergedPath.length;
      return (request) => {
        const url = new URL(request.url);
        url.pathname = url.pathname.slice(pathPrefixLength) || "/";
        return new Request(url, request);
      };
    })();
    const handler2 = async (c, next) => {
      const res = await applicationHandler(replaceRequest(c.req.raw), ...getOptions(c));
      if (res) {
        return res;
      }
      await next();
    };
    this.#addRoute(METHOD_NAME_ALL, mergePath(path2, "*"), handler2);
    return this;
  }
  #addRoute(method, path2, handler2) {
    method = method.toUpperCase();
    path2 = mergePath(this._basePath, path2);
    const r = { basePath: this._basePath, path: path2, method, handler: handler2 };
    this.router.add(method, path2, [handler2, r]);
    this.routes.push(r);
  }
  #handleError(err, c) {
    if (err instanceof Error) {
      return this.errorHandler(err, c);
    }
    throw err;
  }
  #dispatch(request, executionCtx, env, method) {
    if (method === "HEAD") {
      return (async () => new Response(null, await this.#dispatch(request, executionCtx, env, "GET")))();
    }
    const path2 = this.getPath(request, { env });
    const matchResult = this.router.match(method, path2);
    const c = new Context(request, {
      path: path2,
      matchResult,
      env,
      executionCtx,
      notFoundHandler: this.#notFoundHandler
    });
    if (matchResult[0].length === 1) {
      let res;
      try {
        res = matchResult[0][0][0][0](c, async () => {
          c.res = await this.#notFoundHandler(c);
        });
      } catch (err) {
        return this.#handleError(err, c);
      }
      return res instanceof Promise ? res.then(
        (resolved) => resolved || (c.finalized ? c.res : this.#notFoundHandler(c))
      ).catch((err) => this.#handleError(err, c)) : res ?? this.#notFoundHandler(c);
    }
    const composed = compose(matchResult[0], this.errorHandler, this.#notFoundHandler);
    return (async () => {
      try {
        const context = await composed(c);
        if (!context.finalized) {
          throw new Error(
            "Context is not finalized. Did you forget to return a Response object or `await next()`?"
          );
        }
        return context.res;
      } catch (err) {
        return this.#handleError(err, c);
      }
    })();
  }
  /**
   * `.fetch()` will be entry point of your app.
   *
   * @see {@link https://hono.dev/docs/api/hono#fetch}
   *
   * @param {Request} request - request Object of request
   * @param {Env} Env - env Object
   * @param {ExecutionContext} - context of execution
   * @returns {Response | Promise<Response>} response of request
   *
   */
  fetch = (request, ...rest) => {
    return this.#dispatch(request, rest[1], rest[0], request.method);
  };
  /**
   * `.request()` is a useful method for testing.
   * You can pass a URL or pathname to send a GET request.
   * app will return a Response object.
   * ```ts
   * test('GET /hello is ok', async () => {
   *   const res = await app.request('/hello')
   *   expect(res.status).toBe(200)
   * })
   * ```
   * @see https://hono.dev/docs/api/hono#request
   */
  request = (input2, requestInit, Env, executionCtx) => {
    if (input2 instanceof Request) {
      return this.fetch(requestInit ? new Request(input2, requestInit) : input2, Env, executionCtx);
    }
    input2 = input2.toString();
    return this.fetch(
      new Request(
        /^https?:\/\//.test(input2) ? input2 : `http://localhost${mergePath("/", input2)}`,
        requestInit
      ),
      Env,
      executionCtx
    );
  };
  /**
   * `.fire()` automatically adds a global fetch event listener.
   * This can be useful for environments that adhere to the Service Worker API, such as non-ES module Cloudflare Workers.
   * @deprecated
   * Use `fire` from `hono/service-worker` instead.
   * ```ts
   * import { Hono } from 'hono'
   * import { fire } from 'hono/service-worker'
   *
   * const app = new Hono()
   * // ...
   * fire(app)
   * ```
   * @see https://hono.dev/docs/api/hono#fire
   * @see https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API
   * @see https://developers.cloudflare.com/workers/reference/migrate-to-module-workers/
   */
  fire = () => {
    addEventListener("fetch", (event) => {
      event.respondWith(this.#dispatch(event.request, event, void 0, event.request.method));
    });
  };
};

// node_modules/hono/dist/router/reg-exp-router/matcher.js
var emptyParam = [];
function match(method, path2) {
  const matchers = this.buildAllMatchers();
  const match2 = ((method2, path22) => {
    const matcher = matchers[method2] || matchers[METHOD_NAME_ALL];
    const staticMatch = matcher[2][path22];
    if (staticMatch) {
      return staticMatch;
    }
    const match3 = path22.match(matcher[0]);
    if (!match3) {
      return [[], emptyParam];
    }
    const index = match3.indexOf("", 1);
    return [matcher[1][index], match3];
  });
  this.match = match2;
  return match2(method, path2);
}

// node_modules/hono/dist/router/reg-exp-router/node.js
var LABEL_REG_EXP_STR = "[^/]+";
var ONLY_WILDCARD_REG_EXP_STR = ".*";
var TAIL_WILDCARD_REG_EXP_STR = "(?:|/.*)";
var PATH_ERROR = /* @__PURE__ */ Symbol();
var regExpMetaChars = new Set(".\\+*[^]$()");
function compareKey(a, b) {
  if (a.length === 1) {
    return b.length === 1 ? a < b ? -1 : 1 : -1;
  }
  if (b.length === 1) {
    return 1;
  }
  if (a === ONLY_WILDCARD_REG_EXP_STR || a === TAIL_WILDCARD_REG_EXP_STR) {
    return 1;
  } else if (b === ONLY_WILDCARD_REG_EXP_STR || b === TAIL_WILDCARD_REG_EXP_STR) {
    return -1;
  }
  if (a === LABEL_REG_EXP_STR) {
    return 1;
  } else if (b === LABEL_REG_EXP_STR) {
    return -1;
  }
  return a.length === b.length ? a < b ? -1 : 1 : b.length - a.length;
}
var Node = class _Node {
  #index;
  #varIndex;
  #children = /* @__PURE__ */ Object.create(null);
  insert(tokens, index, paramMap, context, pathErrorCheckOnly) {
    if (tokens.length === 0) {
      if (this.#index !== void 0) {
        throw PATH_ERROR;
      }
      if (pathErrorCheckOnly) {
        return;
      }
      this.#index = index;
      return;
    }
    const [token, ...restTokens] = tokens;
    const pattern = token === "*" ? restTokens.length === 0 ? ["", "", ONLY_WILDCARD_REG_EXP_STR] : ["", "", LABEL_REG_EXP_STR] : token === "/*" ? ["", "", TAIL_WILDCARD_REG_EXP_STR] : token.match(/^\:([^\{\}]+)(?:\{(.+)\})?$/);
    let node;
    if (pattern) {
      const name = pattern[1];
      let regexpStr = pattern[2] || LABEL_REG_EXP_STR;
      if (name && pattern[2]) {
        if (regexpStr === ".*") {
          throw PATH_ERROR;
        }
        regexpStr = regexpStr.replace(/^\((?!\?:)(?=[^)]+\)$)/, "(?:");
        if (/\((?!\?:)/.test(regexpStr)) {
          throw PATH_ERROR;
        }
      }
      node = this.#children[regexpStr];
      if (!node) {
        if (Object.keys(this.#children).some(
          (k) => k !== ONLY_WILDCARD_REG_EXP_STR && k !== TAIL_WILDCARD_REG_EXP_STR
        )) {
          throw PATH_ERROR;
        }
        if (pathErrorCheckOnly) {
          return;
        }
        node = this.#children[regexpStr] = new _Node();
        if (name !== "") {
          node.#varIndex = context.varIndex++;
        }
      }
      if (!pathErrorCheckOnly && name !== "") {
        paramMap.push([name, node.#varIndex]);
      }
    } else {
      node = this.#children[token];
      if (!node) {
        if (Object.keys(this.#children).some(
          (k) => k.length > 1 && k !== ONLY_WILDCARD_REG_EXP_STR && k !== TAIL_WILDCARD_REG_EXP_STR
        )) {
          throw PATH_ERROR;
        }
        if (pathErrorCheckOnly) {
          return;
        }
        node = this.#children[token] = new _Node();
      }
    }
    node.insert(restTokens, index, paramMap, context, pathErrorCheckOnly);
  }
  buildRegExpStr() {
    const childKeys = Object.keys(this.#children).sort(compareKey);
    const strList = childKeys.map((k) => {
      const c = this.#children[k];
      return (typeof c.#varIndex === "number" ? `(${k})@${c.#varIndex}` : regExpMetaChars.has(k) ? `\\${k}` : k) + c.buildRegExpStr();
    });
    if (typeof this.#index === "number") {
      strList.unshift(`#${this.#index}`);
    }
    if (strList.length === 0) {
      return "";
    }
    if (strList.length === 1) {
      return strList[0];
    }
    return "(?:" + strList.join("|") + ")";
  }
};

// node_modules/hono/dist/router/reg-exp-router/trie.js
var Trie = class {
  #context = { varIndex: 0 };
  #root = new Node();
  insert(path2, index, pathErrorCheckOnly) {
    const paramAssoc = [];
    const groups = [];
    for (let i = 0; ; ) {
      let replaced = false;
      path2 = path2.replace(/\{[^}]+\}/g, (m) => {
        const mark = `@\\${i}`;
        groups[i] = [mark, m];
        i++;
        replaced = true;
        return mark;
      });
      if (!replaced) {
        break;
      }
    }
    const tokens = path2.match(/(?::[^\/]+)|(?:\/\*$)|./g) || [];
    for (let i = groups.length - 1; i >= 0; i--) {
      const [mark] = groups[i];
      for (let j = tokens.length - 1; j >= 0; j--) {
        if (tokens[j].indexOf(mark) !== -1) {
          tokens[j] = tokens[j].replace(mark, groups[i][1]);
          break;
        }
      }
    }
    this.#root.insert(tokens, index, paramAssoc, this.#context, pathErrorCheckOnly);
    return paramAssoc;
  }
  buildRegExp() {
    let regexp = this.#root.buildRegExpStr();
    if (regexp === "") {
      return [/^$/, [], []];
    }
    let captureIndex = 0;
    const indexReplacementMap = [];
    const paramReplacementMap = [];
    regexp = regexp.replace(/#(\d+)|@(\d+)|\.\*\$/g, (_, handlerIndex, paramIndex) => {
      if (handlerIndex !== void 0) {
        indexReplacementMap[++captureIndex] = Number(handlerIndex);
        return "$()";
      }
      if (paramIndex !== void 0) {
        paramReplacementMap[Number(paramIndex)] = ++captureIndex;
        return "";
      }
      return "";
    });
    return [new RegExp(`^${regexp}`), indexReplacementMap, paramReplacementMap];
  }
};

// node_modules/hono/dist/router/reg-exp-router/router.js
var nullMatcher = [/^$/, [], /* @__PURE__ */ Object.create(null)];
var wildcardRegExpCache = /* @__PURE__ */ Object.create(null);
function buildWildcardRegExp(path2) {
  return wildcardRegExpCache[path2] ??= new RegExp(
    path2 === "*" ? "" : `^${path2.replace(
      /\/\*$|([.\\+*[^\]$()])/g,
      (_, metaChar) => metaChar ? `\\${metaChar}` : "(?:|/.*)"
    )}$`
  );
}
function clearWildcardRegExpCache() {
  wildcardRegExpCache = /* @__PURE__ */ Object.create(null);
}
function buildMatcherFromPreprocessedRoutes(routes) {
  const trie = new Trie();
  const handlerData = [];
  if (routes.length === 0) {
    return nullMatcher;
  }
  const routesWithStaticPathFlag = routes.map(
    (route) => [!/\*|\/:/.test(route[0]), ...route]
  ).sort(
    ([isStaticA, pathA], [isStaticB, pathB]) => isStaticA ? 1 : isStaticB ? -1 : pathA.length - pathB.length
  );
  const staticMap = /* @__PURE__ */ Object.create(null);
  for (let i = 0, j = -1, len = routesWithStaticPathFlag.length; i < len; i++) {
    const [pathErrorCheckOnly, path2, handlers] = routesWithStaticPathFlag[i];
    if (pathErrorCheckOnly) {
      staticMap[path2] = [handlers.map(([h]) => [h, /* @__PURE__ */ Object.create(null)]), emptyParam];
    } else {
      j++;
    }
    let paramAssoc;
    try {
      paramAssoc = trie.insert(path2, j, pathErrorCheckOnly);
    } catch (e) {
      throw e === PATH_ERROR ? new UnsupportedPathError(path2) : e;
    }
    if (pathErrorCheckOnly) {
      continue;
    }
    handlerData[j] = handlers.map(([h, paramCount]) => {
      const paramIndexMap = /* @__PURE__ */ Object.create(null);
      paramCount -= 1;
      for (; paramCount >= 0; paramCount--) {
        const [key, value] = paramAssoc[paramCount];
        paramIndexMap[key] = value;
      }
      return [h, paramIndexMap];
    });
  }
  const [regexp, indexReplacementMap, paramReplacementMap] = trie.buildRegExp();
  for (let i = 0, len = handlerData.length; i < len; i++) {
    for (let j = 0, len2 = handlerData[i].length; j < len2; j++) {
      const map = handlerData[i][j]?.[1];
      if (!map) {
        continue;
      }
      const keys = Object.keys(map);
      for (let k = 0, len3 = keys.length; k < len3; k++) {
        map[keys[k]] = paramReplacementMap[map[keys[k]]];
      }
    }
  }
  const handlerMap = [];
  for (const i in indexReplacementMap) {
    handlerMap[i] = handlerData[indexReplacementMap[i]];
  }
  return [regexp, handlerMap, staticMap];
}
function findMiddleware(middleware, path2) {
  if (!middleware) {
    return void 0;
  }
  for (const k of Object.keys(middleware).sort((a, b) => b.length - a.length)) {
    if (buildWildcardRegExp(k).test(path2)) {
      return [...middleware[k]];
    }
  }
  return void 0;
}
var RegExpRouter = class {
  name = "RegExpRouter";
  #middleware;
  #routes;
  constructor() {
    this.#middleware = { [METHOD_NAME_ALL]: /* @__PURE__ */ Object.create(null) };
    this.#routes = { [METHOD_NAME_ALL]: /* @__PURE__ */ Object.create(null) };
  }
  add(method, path2, handler2) {
    const middleware = this.#middleware;
    const routes = this.#routes;
    if (!middleware || !routes) {
      throw new Error(MESSAGE_MATCHER_IS_ALREADY_BUILT);
    }
    if (!middleware[method]) {
      ;
      [middleware, routes].forEach((handlerMap) => {
        handlerMap[method] = /* @__PURE__ */ Object.create(null);
        Object.keys(handlerMap[METHOD_NAME_ALL]).forEach((p) => {
          handlerMap[method][p] = [...handlerMap[METHOD_NAME_ALL][p]];
        });
      });
    }
    if (path2 === "/*") {
      path2 = "*";
    }
    const paramCount = (path2.match(/\/:/g) || []).length;
    if (/\*$/.test(path2)) {
      const re = buildWildcardRegExp(path2);
      if (method === METHOD_NAME_ALL) {
        Object.keys(middleware).forEach((m) => {
          middleware[m][path2] ||= findMiddleware(middleware[m], path2) || findMiddleware(middleware[METHOD_NAME_ALL], path2) || [];
        });
      } else {
        middleware[method][path2] ||= findMiddleware(middleware[method], path2) || findMiddleware(middleware[METHOD_NAME_ALL], path2) || [];
      }
      Object.keys(middleware).forEach((m) => {
        if (method === METHOD_NAME_ALL || method === m) {
          Object.keys(middleware[m]).forEach((p) => {
            re.test(p) && middleware[m][p].push([handler2, paramCount]);
          });
        }
      });
      Object.keys(routes).forEach((m) => {
        if (method === METHOD_NAME_ALL || method === m) {
          Object.keys(routes[m]).forEach(
            (p) => re.test(p) && routes[m][p].push([handler2, paramCount])
          );
        }
      });
      return;
    }
    const paths = checkOptionalParameter(path2) || [path2];
    for (let i = 0, len = paths.length; i < len; i++) {
      const path22 = paths[i];
      Object.keys(routes).forEach((m) => {
        if (method === METHOD_NAME_ALL || method === m) {
          routes[m][path22] ||= [
            ...findMiddleware(middleware[m], path22) || findMiddleware(middleware[METHOD_NAME_ALL], path22) || []
          ];
          routes[m][path22].push([handler2, paramCount - len + i + 1]);
        }
      });
    }
  }
  match = match;
  buildAllMatchers() {
    const matchers = /* @__PURE__ */ Object.create(null);
    Object.keys(this.#routes).concat(Object.keys(this.#middleware)).forEach((method) => {
      matchers[method] ||= this.#buildMatcher(method);
    });
    this.#middleware = this.#routes = void 0;
    clearWildcardRegExpCache();
    return matchers;
  }
  #buildMatcher(method) {
    const routes = [];
    let hasOwnRoute = method === METHOD_NAME_ALL;
    [this.#middleware, this.#routes].forEach((r) => {
      const ownRoute = r[method] ? Object.keys(r[method]).map((path2) => [path2, r[method][path2]]) : [];
      if (ownRoute.length !== 0) {
        hasOwnRoute ||= true;
        routes.push(...ownRoute);
      } else if (method !== METHOD_NAME_ALL) {
        routes.push(
          ...Object.keys(r[METHOD_NAME_ALL]).map((path2) => [path2, r[METHOD_NAME_ALL][path2]])
        );
      }
    });
    if (!hasOwnRoute) {
      return null;
    } else {
      return buildMatcherFromPreprocessedRoutes(routes);
    }
  }
};

// node_modules/hono/dist/router/smart-router/router.js
var SmartRouter = class {
  name = "SmartRouter";
  #routers = [];
  #routes = [];
  constructor(init) {
    this.#routers = init.routers;
  }
  add(method, path2, handler2) {
    if (!this.#routes) {
      throw new Error(MESSAGE_MATCHER_IS_ALREADY_BUILT);
    }
    this.#routes.push([method, path2, handler2]);
  }
  match(method, path2) {
    if (!this.#routes) {
      throw new Error("Fatal error");
    }
    const routers = this.#routers;
    const routes = this.#routes;
    const len = routers.length;
    let i = 0;
    let res;
    for (; i < len; i++) {
      const router = routers[i];
      try {
        for (let i2 = 0, len2 = routes.length; i2 < len2; i2++) {
          router.add(...routes[i2]);
        }
        res = router.match(method, path2);
      } catch (e) {
        if (e instanceof UnsupportedPathError) {
          continue;
        }
        throw e;
      }
      this.match = router.match.bind(router);
      this.#routers = [router];
      this.#routes = void 0;
      break;
    }
    if (i === len) {
      throw new Error("Fatal error");
    }
    this.name = `SmartRouter + ${this.activeRouter.name}`;
    return res;
  }
  get activeRouter() {
    if (this.#routes || this.#routers.length !== 1) {
      throw new Error("No active router has been determined yet.");
    }
    return this.#routers[0];
  }
};

// node_modules/hono/dist/router/trie-router/node.js
var emptyParams = /* @__PURE__ */ Object.create(null);
var hasChildren = (children) => {
  for (const _ in children) {
    return true;
  }
  return false;
};
var Node2 = class _Node2 {
  #methods;
  #children;
  #patterns;
  #order = 0;
  #params = emptyParams;
  constructor(method, handler2, children) {
    this.#children = children || /* @__PURE__ */ Object.create(null);
    this.#methods = [];
    if (method && handler2) {
      const m = /* @__PURE__ */ Object.create(null);
      m[method] = { handler: handler2, possibleKeys: [], score: 0 };
      this.#methods = [m];
    }
    this.#patterns = [];
  }
  insert(method, path2, handler2) {
    this.#order = ++this.#order;
    let curNode = this;
    const parts = splitRoutingPath(path2);
    const possibleKeys = [];
    for (let i = 0, len = parts.length; i < len; i++) {
      const p = parts[i];
      const nextP = parts[i + 1];
      const pattern = getPattern(p, nextP);
      const key = Array.isArray(pattern) ? pattern[0] : p;
      if (key in curNode.#children) {
        curNode = curNode.#children[key];
        if (pattern) {
          possibleKeys.push(pattern[1]);
        }
        continue;
      }
      curNode.#children[key] = new _Node2();
      if (pattern) {
        curNode.#patterns.push(pattern);
        possibleKeys.push(pattern[1]);
      }
      curNode = curNode.#children[key];
    }
    curNode.#methods.push({
      [method]: {
        handler: handler2,
        possibleKeys: possibleKeys.filter((v, i, a) => a.indexOf(v) === i),
        score: this.#order
      }
    });
    return curNode;
  }
  #pushHandlerSets(handlerSets, node, method, nodeParams, params) {
    for (let i = 0, len = node.#methods.length; i < len; i++) {
      const m = node.#methods[i];
      const handlerSet = m[method] || m[METHOD_NAME_ALL];
      const processedSet = {};
      if (handlerSet !== void 0) {
        handlerSet.params = /* @__PURE__ */ Object.create(null);
        handlerSets.push(handlerSet);
        if (nodeParams !== emptyParams || params && params !== emptyParams) {
          for (let i2 = 0, len2 = handlerSet.possibleKeys.length; i2 < len2; i2++) {
            const key = handlerSet.possibleKeys[i2];
            const processed = processedSet[handlerSet.score];
            handlerSet.params[key] = params?.[key] && !processed ? params[key] : nodeParams[key] ?? params?.[key];
            processedSet[handlerSet.score] = true;
          }
        }
      }
    }
  }
  search(method, path2) {
    const handlerSets = [];
    this.#params = emptyParams;
    const curNode = this;
    let curNodes = [curNode];
    const parts = splitPath(path2);
    const curNodesQueue = [];
    const len = parts.length;
    let partOffsets = null;
    for (let i = 0; i < len; i++) {
      const part = parts[i];
      const isLast = i === len - 1;
      const tempNodes = [];
      for (let j = 0, len2 = curNodes.length; j < len2; j++) {
        const node = curNodes[j];
        const nextNode = node.#children[part];
        if (nextNode) {
          nextNode.#params = node.#params;
          if (isLast) {
            if (nextNode.#children["*"]) {
              this.#pushHandlerSets(handlerSets, nextNode.#children["*"], method, node.#params);
            }
            this.#pushHandlerSets(handlerSets, nextNode, method, node.#params);
          } else {
            tempNodes.push(nextNode);
          }
        }
        for (let k = 0, len3 = node.#patterns.length; k < len3; k++) {
          const pattern = node.#patterns[k];
          const params = node.#params === emptyParams ? {} : { ...node.#params };
          if (pattern === "*") {
            const astNode = node.#children["*"];
            if (astNode) {
              this.#pushHandlerSets(handlerSets, astNode, method, node.#params);
              astNode.#params = params;
              tempNodes.push(astNode);
            }
            continue;
          }
          const [key, name, matcher] = pattern;
          if (!part && !(matcher instanceof RegExp)) {
            continue;
          }
          const child = node.#children[key];
          if (matcher instanceof RegExp) {
            if (partOffsets === null) {
              partOffsets = new Array(len);
              let offset = path2[0] === "/" ? 1 : 0;
              for (let p = 0; p < len; p++) {
                partOffsets[p] = offset;
                offset += parts[p].length + 1;
              }
            }
            const restPathString = path2.substring(partOffsets[i]);
            const m = matcher.exec(restPathString);
            if (m) {
              params[name] = m[0];
              this.#pushHandlerSets(handlerSets, child, method, node.#params, params);
              if (hasChildren(child.#children)) {
                child.#params = params;
                const componentCount = m[0].match(/\//)?.length ?? 0;
                const targetCurNodes = curNodesQueue[componentCount] ||= [];
                targetCurNodes.push(child);
              }
              continue;
            }
          }
          if (matcher === true || matcher.test(part)) {
            params[name] = part;
            if (isLast) {
              this.#pushHandlerSets(handlerSets, child, method, params, node.#params);
              if (child.#children["*"]) {
                this.#pushHandlerSets(
                  handlerSets,
                  child.#children["*"],
                  method,
                  params,
                  node.#params
                );
              }
            } else {
              child.#params = params;
              tempNodes.push(child);
            }
          }
        }
      }
      const shifted = curNodesQueue.shift();
      curNodes = shifted ? tempNodes.concat(shifted) : tempNodes;
    }
    if (handlerSets.length > 1) {
      handlerSets.sort((a, b) => {
        return a.score - b.score;
      });
    }
    return [handlerSets.map(({ handler: handler2, params }) => [handler2, params])];
  }
};

// node_modules/hono/dist/router/trie-router/router.js
var TrieRouter = class {
  name = "TrieRouter";
  #node;
  constructor() {
    this.#node = new Node2();
  }
  add(method, path2, handler2) {
    const results = checkOptionalParameter(path2);
    if (results) {
      for (let i = 0, len = results.length; i < len; i++) {
        this.#node.insert(method, results[i], handler2);
      }
      return;
    }
    this.#node.insert(method, path2, handler2);
  }
  match(method, path2) {
    return this.#node.search(method, path2);
  }
};

// node_modules/hono/dist/hono.js
var Hono2 = class extends Hono {
  /**
   * Creates an instance of the Hono class.
   *
   * @param options - Optional configuration options for the Hono instance.
   */
  constructor(options = {}) {
    super(options);
    this.router = options.router ?? new SmartRouter({
      routers: [new RegExpRouter(), new TrieRouter()]
    });
  }
};

// node_modules/hono/dist/utils/stream.js
var StreamingApi = class {
  writer;
  encoder;
  writable;
  abortSubscribers = [];
  responseReadable;
  /**
   * Whether the stream has been aborted.
   */
  aborted = false;
  /**
   * Whether the stream has been closed normally.
   */
  closed = false;
  constructor(writable, _readable) {
    this.writable = writable;
    this.writer = writable.getWriter();
    this.encoder = new TextEncoder();
    const reader = _readable.getReader();
    this.abortSubscribers.push(async () => {
      await reader.cancel();
    });
    this.responseReadable = new ReadableStream({
      async pull(controller) {
        const { done, value } = await reader.read();
        done ? controller.close() : controller.enqueue(value);
      },
      cancel: () => {
        this.abort();
      }
    });
  }
  async write(input2) {
    try {
      if (typeof input2 === "string") {
        input2 = this.encoder.encode(input2);
      }
      await this.writer.write(input2);
    } catch {
    }
    return this;
  }
  async writeln(input2) {
    await this.write(input2 + "\n");
    return this;
  }
  sleep(ms) {
    return new Promise((res) => setTimeout(res, ms));
  }
  async close() {
    try {
      await this.writer.close();
    } catch {
    }
    this.closed = true;
  }
  async pipe(body) {
    this.writer.releaseLock();
    await body.pipeTo(this.writable, { preventClose: true });
    this.writer = this.writable.getWriter();
  }
  onAbort(listener) {
    this.abortSubscribers.push(listener);
  }
  /**
   * Abort the stream.
   * You can call this method when stream is aborted by external event.
   */
  abort() {
    if (!this.aborted) {
      this.aborted = true;
      this.abortSubscribers.forEach((subscriber) => subscriber());
    }
  }
};

// node_modules/hono/dist/helper/streaming/utils.js
var isOldBunVersion = () => {
  const version = typeof Bun !== "undefined" ? Bun.version : void 0;
  if (version === void 0) {
    return false;
  }
  const result = version.startsWith("1.1") || version.startsWith("1.0") || version.startsWith("0.");
  isOldBunVersion = () => result;
  return result;
};

// node_modules/hono/dist/helper/streaming/sse.js
var SSEStreamingApi = class extends StreamingApi {
  constructor(writable, readable) {
    super(writable, readable);
  }
  async writeSSE(message) {
    const data = await resolveCallback(message.data, HtmlEscapedCallbackPhase.Stringify, false, {});
    const dataLines = data.split(/\r\n|\r|\n/).map((line) => {
      return `data: ${line}`;
    }).join("\n");
    for (const key of ["event", "id", "retry"]) {
      if (message[key] && /[\r\n]/.test(message[key])) {
        throw new Error(`${key} must not contain "\\r" or "\\n"`);
      }
    }
    const sseData = [
      message.event && `event: ${message.event}`,
      dataLines,
      message.id && `id: ${message.id}`,
      message.retry && `retry: ${message.retry}`
    ].filter(Boolean).join("\n") + "\n\n";
    await this.write(sseData);
  }
};
var run = async (stream2, cb, onError) => {
  try {
    await cb(stream2);
  } catch (e) {
    if (e instanceof Error && onError) {
      await onError(e, stream2);
      await stream2.writeSSE({
        event: "error",
        data: e.message
      });
    } else {
      console.error(e);
    }
  } finally {
    stream2.close();
  }
};
var contextStash = /* @__PURE__ */ new WeakMap();
var streamSSE = (c, cb, onError) => {
  const { readable, writable } = new TransformStream();
  const stream2 = new SSEStreamingApi(writable, readable);
  if (isOldBunVersion()) {
    c.req.raw.signal.addEventListener("abort", () => {
      if (!stream2.closed) {
        stream2.abort();
      }
    });
  }
  contextStash.set(stream2.responseReadable, c);
  c.header("Transfer-Encoding", "chunked");
  c.header("Content-Type", "text/event-stream");
  c.header("Cache-Control", "no-cache");
  c.header("Connection", "keep-alive");
  run(stream2, cb, onError);
  return c.newResponse(stream2.responseReadable);
};

// src/app.ts
var import_client_cloudwatch_logs = require("@aws-sdk/client-cloudwatch-logs");

// src/discord.ts
var import_discord_interactions = __toESM(require_dist());
var DISCORD_PUBLIC_KEY = process.env.DISCORD_PUBLIC_KEY ?? "";
var DISCORD_BOT_TOKEN = process.env.DISCORD_BOT_TOKEN ?? "";
var DISCORD_APP_ID = process.env.DISCORD_APP_ID ?? "";
function makeDiscordHandler(backend2) {
  return (c) => discordHandler(c, backend2);
}
async function discordHandler(c, backend2) {
  const signature = c.req.header("x-signature-ed25519") ?? "";
  const timestamp = c.req.header("x-signature-timestamp") ?? "";
  const rawBody = await c.req.text();
  const isValid = await (0, import_discord_interactions.verifyKey)(rawBody, signature, timestamp, DISCORD_PUBLIC_KEY);
  if (!isValid) {
    log.warn("discord: invalid signature");
    return c.text("invalid signature", 401);
  }
  const interaction = JSON.parse(rawBody);
  if (interaction.type === import_discord_interactions.InteractionType.PING) {
    return c.json({ type: import_discord_interactions.InteractionResponseType.PONG });
  }
  if (interaction.type === import_discord_interactions.InteractionType.APPLICATION_COMMAND) {
    const name = interaction.data?.name ?? "";
    const gameName = interaction.data?.options?.find((o) => o.name === "game")?.value ?? "";
    const games = backend2.getGames();
    const config = games[gameName];
    if (!config) {
      return c.json({
        type: import_discord_interactions.InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: `Unknown game: \`${gameName}\`` }
      });
    }
    if (name === "status") {
      log.info(`discord: /status ${gameName}`);
      const state = await backend2.getGameState(config);
      return c.json({
        type: import_discord_interactions.InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: formatState(gameName, state) }
      });
    }
    if (name === "stop") {
      log.info(`discord: /stop ${gameName}`);
      const state = await backend2.stopGame(config);
      log.info(`discord: /stop ${gameName} \u2192 ${state.status}`);
      return c.json({
        type: import_discord_interactions.InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
        data: { content: formatState(gameName, state) }
      });
    }
    if (name === "start") {
      log.info(`discord: /start ${gameName}`);
      void sendFollowup(interaction.token, gameName, config, backend2);
      return c.json({ type: import_discord_interactions.InteractionResponseType.DEFERRED_CHANNEL_MESSAGE_WITH_SOURCE });
    }
    return c.json({
      type: import_discord_interactions.InteractionResponseType.CHANNEL_MESSAGE_WITH_SOURCE,
      data: { content: `Unknown command: \`${name}\`` }
    });
  }
  return c.text("unhandled interaction type", 400);
}
async function sendFollowup(interactionToken, gameName, config, backend2) {
  const state = await backend2.startGame(config);
  log.info(`discord: /start ${gameName} \u2192 ${state.status}`);
  const content = formatState(gameName, state);
  await fetch(
    `https://discord.com/api/v10/webhooks/${DISCORD_APP_ID}/${interactionToken}/messages/@original`,
    {
      method: "PATCH",
      headers: {
        "Authorization": `Bot ${DISCORD_BOT_TOKEN}`,
        "Content-Type": "application/json"
      },
      body: JSON.stringify({ content })
    }
  );
}
function formatState(gameName, state) {
  const parts = [`**${gameName}** \u2014 ${state.status}`];
  if (state.publicIp) parts.push(`\`${state.publicIp}\``);
  if (state.players > 0) parts.push(`${state.players} player(s)`);
  return parts.join(" \u2014 ");
}

// node_modules/hono/dist/jsx/constants.js
var DOM_RENDERER = /* @__PURE__ */ Symbol("RENDERER");
var DOM_ERROR_HANDLER = /* @__PURE__ */ Symbol("ERROR_HANDLER");
var DOM_INTERNAL_TAG = /* @__PURE__ */ Symbol("INTERNAL");
var PERMALINK = /* @__PURE__ */ Symbol("PERMALINK");

// node_modules/hono/dist/jsx/dom/utils.js
var setInternalTagFlag = (fn) => {
  ;
  fn[DOM_INTERNAL_TAG] = true;
  return fn;
};

// node_modules/hono/dist/jsx/dom/context.js
var createContextProviderFunction = (values) => ({ value, children }) => {
  if (!children) {
    return void 0;
  }
  const props = {
    children: [
      {
        tag: setInternalTagFlag(() => {
          values.push(value);
        }),
        props: {}
      }
    ]
  };
  if (Array.isArray(children)) {
    props.children.push(...children.flat());
  } else {
    props.children.push(children);
  }
  props.children.push({
    tag: setInternalTagFlag(() => {
      values.pop();
    }),
    props: {}
  });
  const res = { tag: "", props, type: "" };
  res[DOM_ERROR_HANDLER] = (err) => {
    values.pop();
    throw err;
  };
  return res;
};

// node_modules/hono/dist/jsx/context.js
var globalContexts = [];
var createContext = (defaultValue) => {
  const values = [defaultValue];
  const context = ((props) => {
    values.push(props.value);
    let string;
    try {
      string = props.children ? (Array.isArray(props.children) ? new JSXFragmentNode("", {}, props.children) : props.children).toString() : "";
    } catch (e) {
      values.pop();
      throw e;
    }
    if (string instanceof Promise) {
      return string.finally(() => values.pop()).then((resString) => raw(resString, resString.callbacks));
    } else {
      values.pop();
      return raw(string);
    }
  });
  context.values = values;
  context.Provider = context;
  context[DOM_RENDERER] = createContextProviderFunction(values);
  globalContexts.push(context);
  return context;
};
var useContext = (context) => {
  return context.values.at(-1);
};

// node_modules/hono/dist/jsx/intrinsic-element/common.js
var deDupeKeyMap = {
  title: [],
  script: ["src"],
  style: ["data-href"],
  link: ["href"],
  meta: ["name", "httpEquiv", "charset", "itemProp"]
};
var domRenderers = {};
var dataPrecedenceAttr = "data-precedence";
var isStylesheetLinkWithPrecedence = (props) => props.rel === "stylesheet" && "precedence" in props;
var shouldDeDupeByKey = (tagName, supportSort) => {
  if (tagName === "link") {
    return supportSort;
  }
  return deDupeKeyMap[tagName].length > 0;
};

// node_modules/hono/dist/jsx/intrinsic-element/components.js
var components_exports = {};
__export(components_exports, {
  button: () => button,
  form: () => form,
  input: () => input,
  link: () => link,
  meta: () => meta,
  script: () => script,
  style: () => style,
  title: () => title
});

// node_modules/hono/dist/jsx/children.js
var toArray = (children) => Array.isArray(children) ? children : [children];

// node_modules/hono/dist/jsx/intrinsic-element/components.js
var metaTagMap = /* @__PURE__ */ new WeakMap();
var insertIntoHead = (tagName, tag, props, precedence) => ({ buffer, context }) => {
  if (!buffer) {
    return;
  }
  const map = metaTagMap.get(context) || {};
  metaTagMap.set(context, map);
  const tags = map[tagName] ||= [];
  let duped = false;
  const deDupeKeys = deDupeKeyMap[tagName];
  const deDupeByKey = shouldDeDupeByKey(tagName, precedence !== void 0);
  if (deDupeByKey) {
    LOOP: for (const [, tagProps] of tags) {
      if (tagName === "link" && !(tagProps.rel === "stylesheet" && tagProps[dataPrecedenceAttr] !== void 0)) {
        continue;
      }
      for (const key of deDupeKeys) {
        if ((tagProps?.[key] ?? null) === props?.[key]) {
          duped = true;
          break LOOP;
        }
      }
    }
  }
  if (duped) {
    buffer[0] = buffer[0].replaceAll(tag, "");
  } else if (deDupeByKey || tagName === "link") {
    tags.push([tag, props, precedence]);
  } else {
    tags.unshift([tag, props, precedence]);
  }
  if (buffer[0].indexOf("</head>") !== -1) {
    let insertTags;
    if (tagName === "link" || precedence !== void 0) {
      const precedences = [];
      insertTags = tags.map(([tag2, , tagPrecedence], index) => {
        if (tagPrecedence === void 0) {
          return [tag2, Number.MAX_SAFE_INTEGER, index];
        }
        let order = precedences.indexOf(tagPrecedence);
        if (order === -1) {
          precedences.push(tagPrecedence);
          order = precedences.length - 1;
        }
        return [tag2, order, index];
      }).sort((a, b) => a[1] - b[1] || a[2] - b[2]).map(([tag2]) => tag2);
    } else {
      insertTags = tags.map(([tag2]) => tag2);
    }
    insertTags.forEach((tag2) => {
      buffer[0] = buffer[0].replaceAll(tag2, "");
    });
    buffer[0] = buffer[0].replace(/(?=<\/head>)/, insertTags.join(""));
  }
};
var returnWithoutSpecialBehavior = (tag, children, props) => raw(new JSXNode(tag, props, toArray(children ?? [])).toString());
var documentMetadataTag = (tag, children, props, sort) => {
  if ("itemProp" in props) {
    return returnWithoutSpecialBehavior(tag, children, props);
  }
  let { precedence, blocking, ...restProps } = props;
  precedence = sort ? precedence ?? "" : void 0;
  if (sort) {
    restProps[dataPrecedenceAttr] = precedence;
  }
  const string = new JSXNode(tag, restProps, toArray(children || [])).toString();
  if (string instanceof Promise) {
    return string.then(
      (resString) => raw(string, [
        ...resString.callbacks || [],
        insertIntoHead(tag, resString, restProps, precedence)
      ])
    );
  } else {
    return raw(string, [insertIntoHead(tag, string, restProps, precedence)]);
  }
};
var title = ({ children, ...props }) => {
  const nameSpaceContext2 = getNameSpaceContext();
  if (nameSpaceContext2) {
    const context = useContext(nameSpaceContext2);
    if (context === "svg" || context === "head") {
      return new JSXNode(
        "title",
        props,
        toArray(children ?? [])
      );
    }
  }
  return documentMetadataTag("title", children, props, false);
};
var script = ({
  children,
  ...props
}) => {
  const nameSpaceContext2 = getNameSpaceContext();
  if (["src", "async"].some((k) => !props[k]) || nameSpaceContext2 && useContext(nameSpaceContext2) === "head") {
    return returnWithoutSpecialBehavior("script", children, props);
  }
  return documentMetadataTag("script", children, props, false);
};
var style = ({
  children,
  ...props
}) => {
  if (!["href", "precedence"].every((k) => k in props)) {
    return returnWithoutSpecialBehavior("style", children, props);
  }
  props["data-href"] = props.href;
  delete props.href;
  return documentMetadataTag("style", children, props, true);
};
var link = ({ children, ...props }) => {
  if (["onLoad", "onError"].some((k) => k in props) || props.rel === "stylesheet" && (!("precedence" in props) || "disabled" in props)) {
    return returnWithoutSpecialBehavior("link", children, props);
  }
  return documentMetadataTag("link", children, props, isStylesheetLinkWithPrecedence(props));
};
var meta = ({ children, ...props }) => {
  const nameSpaceContext2 = getNameSpaceContext();
  if (nameSpaceContext2 && useContext(nameSpaceContext2) === "head") {
    return returnWithoutSpecialBehavior("meta", children, props);
  }
  return documentMetadataTag("meta", children, props, false);
};
var newJSXNode = (tag, { children, ...props }) => (
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  new JSXNode(tag, props, toArray(children ?? []))
);
var form = (props) => {
  if (typeof props.action === "function") {
    props.action = PERMALINK in props.action ? props.action[PERMALINK] : void 0;
  }
  return newJSXNode("form", props);
};
var formActionableElement = (tag, props) => {
  if (typeof props.formAction === "function") {
    props.formAction = PERMALINK in props.formAction ? props.formAction[PERMALINK] : void 0;
  }
  return newJSXNode(tag, props);
};
var input = (props) => formActionableElement("input", props);
var button = (props) => formActionableElement("button", props);

// node_modules/hono/dist/jsx/utils.js
var normalizeElementKeyMap = /* @__PURE__ */ new Map([
  ["className", "class"],
  ["htmlFor", "for"],
  ["crossOrigin", "crossorigin"],
  ["httpEquiv", "http-equiv"],
  ["itemProp", "itemprop"],
  ["fetchPriority", "fetchpriority"],
  ["noModule", "nomodule"],
  ["formAction", "formaction"]
]);
var normalizeIntrinsicElementKey = (key) => normalizeElementKeyMap.get(key) || key;
var styleObjectForEach = (style2, fn) => {
  for (const [k, v] of Object.entries(style2)) {
    const key = k[0] === "-" || !/[A-Z]/.test(k) ? k : k.replace(/[A-Z]/g, (m) => `-${m.toLowerCase()}`);
    fn(
      key,
      v == null ? null : typeof v === "number" ? !key.match(
        /^(?:a|border-im|column(?:-c|s)|flex(?:$|-[^b])|grid-(?:ar|[^a])|font-w|li|or|sca|st|ta|wido|z)|ty$/
      ) ? `${v}px` : `${v}` : v
    );
  }
};

// node_modules/hono/dist/jsx/base.js
var nameSpaceContext = void 0;
var getNameSpaceContext = () => nameSpaceContext;
var toSVGAttributeName = (key) => /[A-Z]/.test(key) && // Presentation attributes are findable in style object. "clip-path", "font-size", "stroke-width", etc.
// Or other un-deprecated kebab-case attributes. "overline-position", "paint-order", "strikethrough-position", etc.
key.match(
  /^(?:al|basel|clip(?:Path|Rule)$|co|do|fill|fl|fo|gl|let|lig|i|marker[EMS]|o|pai|pointe|sh|st[or]|text[^L]|tr|u|ve|w)/
) ? key.replace(/([A-Z])/g, "-$1").toLowerCase() : key;
var emptyTags = [
  "area",
  "base",
  "br",
  "col",
  "embed",
  "hr",
  "img",
  "input",
  "keygen",
  "link",
  "meta",
  "param",
  "source",
  "track",
  "wbr"
];
var booleanAttributes = [
  "allowfullscreen",
  "async",
  "autofocus",
  "autoplay",
  "checked",
  "controls",
  "default",
  "defer",
  "disabled",
  "download",
  "formnovalidate",
  "hidden",
  "inert",
  "ismap",
  "itemscope",
  "loop",
  "multiple",
  "muted",
  "nomodule",
  "novalidate",
  "open",
  "playsinline",
  "readonly",
  "required",
  "reversed",
  "selected"
];
var childrenToStringToBuffer = (children, buffer) => {
  for (let i = 0, len = children.length; i < len; i++) {
    const child = children[i];
    if (typeof child === "string") {
      escapeToBuffer(child, buffer);
    } else if (typeof child === "boolean" || child === null || child === void 0) {
      continue;
    } else if (child instanceof JSXNode) {
      child.toStringToBuffer(buffer);
    } else if (typeof child === "number" || child.isEscaped) {
      ;
      buffer[0] += child;
    } else if (child instanceof Promise) {
      buffer.unshift("", child);
    } else {
      childrenToStringToBuffer(child, buffer);
    }
  }
};
var JSXNode = class {
  tag;
  props;
  key;
  children;
  isEscaped = true;
  localContexts;
  constructor(tag, props, children) {
    this.tag = tag;
    this.props = props;
    this.children = children;
  }
  get type() {
    return this.tag;
  }
  // Added for compatibility with libraries that rely on React's internal structure
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  get ref() {
    return this.props.ref || null;
  }
  toString() {
    const buffer = [""];
    this.localContexts?.forEach(([context, value]) => {
      context.values.push(value);
    });
    try {
      this.toStringToBuffer(buffer);
    } finally {
      this.localContexts?.forEach(([context]) => {
        context.values.pop();
      });
    }
    return buffer.length === 1 ? "callbacks" in buffer ? resolveCallbackSync(raw(buffer[0], buffer.callbacks)).toString() : buffer[0] : stringBufferToString(buffer, buffer.callbacks);
  }
  toStringToBuffer(buffer) {
    const tag = this.tag;
    const props = this.props;
    let { children } = this;
    buffer[0] += `<${tag}`;
    const normalizeKey = nameSpaceContext && useContext(nameSpaceContext) === "svg" ? (key) => toSVGAttributeName(normalizeIntrinsicElementKey(key)) : (key) => normalizeIntrinsicElementKey(key);
    for (let [key, v] of Object.entries(props)) {
      key = normalizeKey(key);
      if (key === "children") {
      } else if (key === "style" && typeof v === "object") {
        let styleStr = "";
        styleObjectForEach(v, (property, value) => {
          if (value != null) {
            styleStr += `${styleStr ? ";" : ""}${property}:${value}`;
          }
        });
        buffer[0] += ' style="';
        escapeToBuffer(styleStr, buffer);
        buffer[0] += '"';
      } else if (typeof v === "string") {
        buffer[0] += ` ${key}="`;
        escapeToBuffer(v, buffer);
        buffer[0] += '"';
      } else if (v === null || v === void 0) {
      } else if (typeof v === "number" || v.isEscaped) {
        buffer[0] += ` ${key}="${v}"`;
      } else if (typeof v === "boolean" && booleanAttributes.includes(key)) {
        if (v) {
          buffer[0] += ` ${key}=""`;
        }
      } else if (key === "dangerouslySetInnerHTML") {
        if (children.length > 0) {
          throw new Error("Can only set one of `children` or `props.dangerouslySetInnerHTML`.");
        }
        children = [raw(v.__html)];
      } else if (v instanceof Promise) {
        buffer[0] += ` ${key}="`;
        buffer.unshift('"', v);
      } else if (typeof v === "function") {
        if (!key.startsWith("on") && key !== "ref") {
          throw new Error(`Invalid prop '${key}' of type 'function' supplied to '${tag}'.`);
        }
      } else {
        buffer[0] += ` ${key}="`;
        escapeToBuffer(v.toString(), buffer);
        buffer[0] += '"';
      }
    }
    if (emptyTags.includes(tag) && children.length === 0) {
      buffer[0] += "/>";
      return;
    }
    buffer[0] += ">";
    childrenToStringToBuffer(children, buffer);
    buffer[0] += `</${tag}>`;
  }
};
var JSXFunctionNode = class extends JSXNode {
  toStringToBuffer(buffer) {
    const { children } = this;
    const props = { ...this.props };
    if (children.length) {
      props.children = children.length === 1 ? children[0] : children;
    }
    const res = this.tag.call(null, props);
    if (typeof res === "boolean" || res == null) {
      return;
    } else if (res instanceof Promise) {
      if (globalContexts.length === 0) {
        buffer.unshift("", res);
      } else {
        const currentContexts = globalContexts.map((c) => [c, c.values.at(-1)]);
        buffer.unshift(
          "",
          res.then((childRes) => {
            if (childRes instanceof JSXNode) {
              childRes.localContexts = currentContexts;
            }
            return childRes;
          })
        );
      }
    } else if (res instanceof JSXNode) {
      res.toStringToBuffer(buffer);
    } else if (typeof res === "number" || res.isEscaped) {
      buffer[0] += res;
      if (res.callbacks) {
        buffer.callbacks ||= [];
        buffer.callbacks.push(...res.callbacks);
      }
    } else {
      escapeToBuffer(res, buffer);
    }
  }
};
var JSXFragmentNode = class extends JSXNode {
  toStringToBuffer(buffer) {
    childrenToStringToBuffer(this.children, buffer);
  }
};
var initDomRenderer = false;
var jsxFn = (tag, props, children) => {
  if (!initDomRenderer) {
    for (const k in domRenderers) {
      ;
      components_exports[k][DOM_RENDERER] = domRenderers[k];
    }
    initDomRenderer = true;
  }
  if (typeof tag === "function") {
    return new JSXFunctionNode(tag, props, children);
  } else if (components_exports[tag]) {
    return new JSXFunctionNode(
      components_exports[tag],
      props,
      children
    );
  } else if (tag === "svg" || tag === "head") {
    nameSpaceContext ||= createContext("");
    return new JSXNode(tag, props, [
      new JSXFunctionNode(
        nameSpaceContext,
        {
          value: tag
        },
        children
      )
    ]);
  } else {
    return new JSXNode(tag, props, children);
  }
};

// node_modules/hono/dist/jsx/jsx-dev-runtime.js
function jsxDEV(tag, props, key) {
  let node;
  if (!props || !("children" in props)) {
    node = jsxFn(tag, props, []);
  } else {
    const children = props.children;
    node = Array.isArray(children) ? jsxFn(tag, props, children) : jsxFn(tag, props, [children]);
  }
  node.key = key;
  return node;
}

// src/ui.tsx
var SESSION_KEY = "insta-game-passphrase";
var css = `
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { background: #111; color: #eee; font-family: monospace; padding: 2rem; }
  .title-bar { display: flex; align-items: center; gap: 1.5rem; margin-bottom: 2rem; flex-wrap: wrap; }
  h1 { font-size: 1.4rem; }
  #auth-form { display: flex; align-items: center; gap: 0.5rem; }
  #auth-form input { padding: 0.35rem 0.5rem; background: #222; color: #eee; border: 1px solid #444; font-family: monospace; font-size: 0.85rem; width: 12rem; }
  #auth-form button { padding: 0.35rem 0.7rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; font-size: 0.85rem; }
  #auth-status { font-size: 0.8rem; color: #aaa; }

  .accordion { display: flex; flex-direction: column; gap: 0.5rem; }

  .row { border: 1px solid #333; background: #1a1a1a; }
  .row-header {
    display: flex; align-items: center; gap: 1rem;
    padding: 0.75rem 1rem; cursor: pointer; user-select: none;
    width: 100%;
  }
  .row-header:hover { background: #222; }
  .status-dot { font-size: 0.8rem; flex-shrink: 0; }
  .game-name { font-weight: bold; min-width: 8rem; }
  .row-meta { display: flex; gap: 1.5rem; flex: 1; color: #aaa; font-size: 0.85rem; flex-wrap: wrap; }
  .row-meta .online { color: #4f4; }
  .row-meta .starting { color: #fa4; }
  .row-meta .offline { color: #666; }
  .expand-btn {
    background: none; border: none; color: #aaa; cursor: pointer;
    font-family: monospace; font-size: 0.85rem; padding: 0; flex-shrink: 0;
  }

  .row-body { border-top: 1px solid #333; padding: 1rem; display: none; }
  .row-body.open { display: block; }

  .row-details { display: flex; gap: 2rem; align-items: flex-start; flex-wrap: wrap; margin-bottom: 1rem; }
  .connect code { background: #222; padding: 0.2rem 0.5rem; border: 1px solid #444; cursor: pointer; }
  .connect code:hover { background: #2a2a2a; }
  .client-link { font-size: 0.85rem; color: #aaa; }
  .client-link a { color: #88f; text-decoration: none; }
  .client-link a:hover { text-decoration: underline; }

  .admin-section { margin-top: 0.75rem; border-top: 1px solid #222; padding-top: 0.75rem; display: none; }
  .admin-section.unlocked { display: block; }
  .admin-controls { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: center; }
  .admin-controls button { padding: 0.4rem 0.8rem; background: #333; color: #eee; border: 1px solid #555; cursor: pointer; font-family: monospace; }
  .admin-controls button:hover { background: #444; }

  .status-frag { font-size: 0.85rem; color: #aaa; margin-top: 0.5rem; min-height: 1.4em; }
  .status-frag .online { color: #4f4; }
  .status-frag .starting { color: #fa4; }
  .htmx-indicator { opacity: 0; transition: opacity 200ms ease-in; }
  .htmx-request .htmx-indicator { opacity: 1; }

  .log-section { margin-top: 0.75rem; border-top: 1px solid #222; padding-top: 0.75rem; display: none; }
  .log-section.open { display: block; }
  .log-panel { height: 300px; overflow-y: scroll; background: #0a0a0a; font-size: 0.75rem; padding: 0.75rem; border: 1px solid #222; }
  .log-line { white-space: pre-wrap; word-break: break-all; line-height: 1.5; }
  .term-fg1 { font-weight: bold; }
  .term-fg2 { color: #838887; }
  .term-fg3 { font-style: italic; }
  .term-fg4 { text-decoration: underline; }
  .term-fg30 { color: #666; }
  .term-fg31 { color: #ff7070; }
  .term-fg32 { color: #b0f986; }
  .term-fg33 { color: #c6c502; }
  .term-fg34 { color: #8db7e0; }
  .term-fg35 { color: #f271fb; }
  .term-fg36 { color: #6bf7ff; }
  .term-fg37 { color: #eee; }
  .term-fgi90 { color: #838887; }
  .term-fgi91 { color: #ff3333; }
  .term-fgi92 { color: #00ff00; }
  .term-fgi93 { color: #fffc67; }
  .term-fgi94 { color: #6871ff; }
  .term-fgi95 { color: #ff76ff; }
  .term-fgi96 { color: #60fcff; }
`;
var initScript = `
(function() {
  var SESSION_KEY = ${JSON.stringify(SESSION_KEY)};
  var STATUS_POLL_INTERVAL_MS = 10000;
  var STATUS_RETRY_INTERVAL_MS = 30000;
  var LOG_POLL_INTERVAL_MS = 5000;
  var LOG_RETRY_INTERVAL_MS = 15000;
  var POLL_PAUSE_AFTER_ADMIN_MS = 15000;
  var suspendPollingUntil = 0;

  function getPassphrase() {
    return sessionStorage.getItem(SESSION_KEY) || "";
  }

  function statusDot(status) {
    if (status === "online") return "\u{1F7E2}";
    if (status === "starting") return "\u{1F7E1}";
    return "\u26AB";
  }

  function escapeHtml(text) {
    return String(text)
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll(">", "&gt;");
  }

  function renderRowHeader(game, state) {
    var meta = [];
    if (state.status === "online" && state.hostname) meta.push("<span>" + escapeHtml(state.hostname) + "</span>");
    if (state.status === "online" && state.map) meta.push("<span>" + escapeHtml(state.map) + "</span>");
    if (state.status === "online") meta.push("<span>" + state.players + " player" + (state.players !== 1 ? "s" : "") + "</span>");
    if (state.status !== "online") meta.push("<span class=\\"" + state.status + "\\">" + state.status + "</span>");
    return ""
      + "<span class=\\"status-dot\\">" + statusDot(state.status) + "</span>"
      + "<span class=\\"game-name\\">" + escapeHtml(game) + "</span>"
      + "<span class=\\"row-meta\\">" + meta.join("") + "</span>"
      + "<button class=\\"expand-btn\\" id=\\"expand-btn-" + game + "\\">" + "[expand \u25BC]" + "</button>";
  }

  function syncExpandButton(game) {
    var body = document.getElementById("row-body-" + game);
    var btn = document.getElementById("expand-btn-" + game);
    if (!body || !btn) return;
    btn.textContent = body.classList.contains("open") ? "[collapse \u25B2]" : "[expand \u25BC]";
  }

  function refreshStatuses() {
    if (Date.now() < suspendPollingUntil) {
      window.setTimeout(refreshStatuses, Math.max(1000, suspendPollingUntil - Date.now()));
      return;
    }
    var retryDelay = STATUS_POLL_INTERVAL_MS;
    fetch("/status")
      .then(function(res) {
        if (!res.ok) {
          var error = new Error("HTTP " + res.status);
          error.status = res.status;
          throw error;
        }
        return res.json();
      })
      .then(function(payload) {
        Object.entries(payload).forEach(function(entry) {
          var game = entry[0];
          var state = entry[1];
          var header = document.getElementById("row-header-" + game);
          if (!header) return;
          header.innerHTML = renderRowHeader(game, state);
          syncExpandButton(game);
        });
      })
      .catch(function(error) {
        if (error.status === 429) retryDelay = STATUS_RETRY_INTERVAL_MS;
      })
      .finally(function() {
        var delay = retryDelay;
        if (Date.now() < suspendPollingUntil) {
          delay = Math.max(1000, suspendPollingUntil - Date.now());
        }
        window.setTimeout(refreshStatuses, delay);
      });
  }

  function appendLogLines(inner, lines) {
    lines.forEach(function(line) {
      var div = document.createElement("div");
      div.className = "log-line";
      div.textContent = line;
      inner.appendChild(div);
    });
    inner.scrollTop = inner.scrollHeight;
  }

  function pollLogs(game, inner) {
    if (inner.getAttribute("data-log-mode") !== "poll") return;
    if (inner.getAttribute("data-log-open") !== "1") return;
    if (Date.now() < suspendPollingUntil) {
      window.setTimeout(function() { pollLogs(game, inner); }, Math.max(1000, suspendPollingUntil - Date.now()));
      return;
    }
    var pp = getPassphrase();
    var cursor = inner.getAttribute("data-log-cursor") || "";
    var url = inner.getAttribute("data-log-url");
    var retryDelay = LOG_POLL_INTERVAL_MS;
    if (!url) return;
    var sep = url.indexOf("?") >= 0 ? "&" : "?";
    fetch(url + sep + "token=" + encodeURIComponent(pp) + (cursor ? "&cursor=" + encodeURIComponent(cursor) : ""))
      .then(function(res) {
        if (!res.ok) {
          var error = new Error("HTTP " + res.status);
          error.status = res.status;
          throw error;
        }
        return res.json();
      })
      .then(function(payload) {
        if (Array.isArray(payload.lines) && payload.lines.length > 0) {
          appendLogLines(inner, payload.lines);
        }
        if (payload.cursor) inner.setAttribute("data-log-cursor", payload.cursor);
      })
      .catch(function(error) {
        if (error.status === 429) retryDelay = LOG_RETRY_INTERVAL_MS;
        if (error.status !== 429) {
          appendLogLines(inner, ["[log poll error: " + error.message + "]"]);
        }
      })
      .finally(function() {
        if (inner.getAttribute("data-log-open") === "1") {
          var delay = retryDelay;
          if (Date.now() < suspendPollingUntil) {
            delay = Math.max(1000, suspendPollingUntil - Date.now());
          }
          window.setTimeout(function() { pollLogs(game, inner); }, delay);
        }
      });
  }

  // Toggle accordion open/close
  window.toggleRow = function(game) {
    var body = document.getElementById("row-body-" + game);
    var btn = document.getElementById("expand-btn-" + game);
    var open = body.classList.toggle("open");
    btn.textContent = open ? "[collapse \u25B2]" : "[expand \u25BC]";
  };

  // Copy connect address to clipboard
  window.copyConnect = function(text) {
    navigator.clipboard.writeText(text).catch(function() {});
  };

  // Unlock all admin sections \u2014 set hx-headers then show them
  function unlockAll(pp) {
    document.querySelectorAll(".admin-section").forEach(function(section) {
      section.setAttribute("hx-headers", JSON.stringify({"X-Passphrase": pp}));
      section.classList.add("unlocked");
      htmx.process(section);
    });
    document.getElementById("auth-form").style.display = "none";
    var status = document.getElementById("auth-status");
    status.style.display = "";
    status.textContent = "admin";
  }

  document.addEventListener("click", function(event) {
    var target = event.target;
    if (!(target instanceof Element)) return;
    if (!target.closest("[data-admin-action]")) return;
    suspendPollingUntil = Date.now() + POLL_PAUSE_AFTER_ADMIN_MS;
  });

  // Top-level authenticate
  window.authenticate = function() {
    var input = document.getElementById("passphrase-input");
    var btn = document.getElementById("passphrase-btn");
    var val = input.value;
    if (!val) return;
    btn.disabled = true;
    btn.textContent = "checking...";
    fetch("/", { headers: { "X-Passphrase": val, "HX-Request": "true" } })
      .then(function(res) {
        if (res.status === 401) {
          btn.disabled = false;
          btn.textContent = "unlock";
          input.style.borderColor = "#f44";
          return;
        }
        sessionStorage.setItem(SESSION_KEY, val);
        unlockAll(val);
      })
      .catch(function() {
        btn.disabled = false;
        btn.textContent = "unlock";
      });
  };

  // Toggle inline log panel open/closed
  window.toggleLogs = function(game) {
    var section = document.getElementById("log-section-" + game);
    var inner = document.getElementById("log-sse-" + game);
    var isOpen = section.classList.toggle("open");
    inner.setAttribute("data-log-open", isOpen ? "1" : "0");
    if (isOpen) {
      if (inner.getAttribute("data-log-mode") === "poll") {
        if (!inner.getAttribute("data-log-started")) {
          inner.setAttribute("data-log-started", "1");
          appendLogLines(inner, ["[connecting to " + game + " logs]"]);
          pollLogs(game, inner);
        }
      } else if (!inner.getAttribute("sse-connect")) {
        var pp = getPassphrase();
        var baseUrl = inner.getAttribute("data-log-url");
        var separator = baseUrl && baseUrl.indexOf("?") >= 0 ? "&" : "?";
        inner.setAttribute("hx-ext", "sse");
        inner.setAttribute("sse-connect", (baseUrl || "/logs?game=" + game) + separator + "token=" + encodeURIComponent(pp));
        htmx.process(inner);
        var observer = new MutationObserver(function() { inner.scrollTop = inner.scrollHeight; });
        observer.observe(inner, { childList: true });
      }
    }
  };

  // Restore auth on page load
  (function() {
    refreshStatuses();
    var pp = getPassphrase();
    if (!pp) return;
    fetch("/", { headers: { "X-Passphrase": pp, "HX-Request": "true" } })
      .then(function(res) {
        if (res.status === 401) { sessionStorage.removeItem(SESSION_KEY); return; }
        unlockAll(pp);
      });
  })();
})();
`;
var StatusDot = ({ status }) => {
  if (status === "online") return /* @__PURE__ */ jsxDEV("span", { class: "status-dot", children: "\u{1F7E2}" });
  if (status === "starting") return /* @__PURE__ */ jsxDEV("span", { class: "status-dot", children: "\u{1F7E1}" });
  return /* @__PURE__ */ jsxDEV("span", { class: "status-dot", children: "\u26AB" });
};
var AccordionRow = ({ game, displayName, state, connectAddress, clientDownloadUrl, startBlocked, logsEnabled, logMode, logUrl }) => {
  const metaOnline = state.status === "online";
  const indicator = `#status-result-${game}`;
  return /* @__PURE__ */ jsxDEV("div", { class: "row", id: `row-${game}`, children: [
    /* @__PURE__ */ jsxDEV(
      "div",
      {
        id: `row-header-${game}`,
        class: "row-header",
        onclick: `toggleRow('${game}')`,
        children: [
          /* @__PURE__ */ jsxDEV(StatusDot, { status: state.status }),
          /* @__PURE__ */ jsxDEV("span", { class: "game-name", children: displayName ?? game }),
          /* @__PURE__ */ jsxDEV("span", { class: "row-meta", children: [
            metaOnline && state.hostname ? /* @__PURE__ */ jsxDEV("span", { children: state.hostname }) : null,
            metaOnline && state.map ? /* @__PURE__ */ jsxDEV("span", { children: state.map }) : null,
            metaOnline ? /* @__PURE__ */ jsxDEV("span", { children: [
              state.players,
              " player",
              state.players !== 1 ? "s" : ""
            ] }) : null,
            !metaOnline ? /* @__PURE__ */ jsxDEV("span", { class: state.status, children: state.status }) : null
          ] }),
          /* @__PURE__ */ jsxDEV("button", { class: "expand-btn", id: `expand-btn-${game}`, children: "[expand \u25BC]" })
        ]
      }
    ),
    /* @__PURE__ */ jsxDEV("div", { class: "row-body", id: `row-body-${game}`, children: [
      /* @__PURE__ */ jsxDEV("div", { class: "row-details", children: [
        connectAddress ? /* @__PURE__ */ jsxDEV("div", { class: "connect", children: [
          "connect: ",
          /* @__PURE__ */ jsxDEV("code", { onclick: `copyConnect(${JSON.stringify(connectAddress)})`, title: "click to copy", children: connectAddress })
        ] }) : null,
        clientDownloadUrl ? /* @__PURE__ */ jsxDEV("div", { class: "client-link", children: /* @__PURE__ */ jsxDEV("a", { href: clientDownloadUrl, target: "_blank", rel: "noopener", children: "get client \u2197" }) }) : null
      ] }),
      /* @__PURE__ */ jsxDEV("div", { class: "admin-section", id: `admin-section-${game}`, children: [
        /* @__PURE__ */ jsxDEV("div", { class: "admin-controls", children: [
          /* @__PURE__ */ jsxDEV(
            "button",
            {
              "hx-post": `/?game=${game}&operation=start`,
              "data-admin-action": "start",
              "hx-target": indicator,
              "hx-indicator": indicator,
              "hx-disabled-elt": "this",
              disabled: startBlocked || void 0,
              title: startBlocked ? "a conflicting game is already running on the same port" : void 0,
              children: "start"
            }
          ),
          /* @__PURE__ */ jsxDEV(
            "button",
            {
              "hx-post": `/?game=${game}&operation=stop`,
              "data-admin-action": "stop",
              "hx-target": indicator,
              "hx-indicator": indicator,
              "hx-disabled-elt": "this",
              children: "stop"
            }
          ),
          logsEnabled ? /* @__PURE__ */ jsxDEV("button", { type: "button", onclick: `toggleLogs('${game}')`, children: "logs" }) : null
        ] }),
        /* @__PURE__ */ jsxDEV("div", { id: `status-result-${game}`, class: "status-frag", children: /* @__PURE__ */ jsxDEV("span", { class: "htmx-indicator", children: "working..." }) })
      ] }),
      /* @__PURE__ */ jsxDEV("div", { class: "log-section", id: `log-section-${game}`, children: /* @__PURE__ */ jsxDEV(
        "div",
        {
          id: `log-sse-${game}`,
          class: "log-panel",
          "data-log-mode": logMode,
          "data-log-open": "0",
          "data-log-url": logUrl,
          "data-log-cursor": "",
          "sse-swap": "log",
          "hx-swap": "beforeend"
        }
      ) })
    ] })
  ] });
};
function renderUi(games) {
  const page = /* @__PURE__ */ jsxDEV("html", { lang: "en", children: [
    /* @__PURE__ */ jsxDEV("head", { children: [
      /* @__PURE__ */ jsxDEV("meta", { charset: "utf-8" }),
      /* @__PURE__ */ jsxDEV("meta", { name: "viewport", content: "width=device-width, initial-scale=1" }),
      /* @__PURE__ */ jsxDEV("title", { children: "insta-game" }),
      /* @__PURE__ */ jsxDEV("style", { children: css })
    ] }),
    /* @__PURE__ */ jsxDEV("body", { children: [
      /* @__PURE__ */ jsxDEV("div", { class: "title-bar", children: [
        /* @__PURE__ */ jsxDEV("h1", { children: "insta-game" }),
        /* @__PURE__ */ jsxDEV("form", { id: "auth-form", onsubmit: "authenticate(); return false;", children: [
          /* @__PURE__ */ jsxDEV(
            "input",
            {
              type: "text",
              id: "passphrase-input",
              placeholder: "passphrase",
              autocomplete: "off",
              spellcheck: false,
              style: "letter-spacing:0.15em;",
              oninput: "this.style.borderColor=''"
            }
          ),
          /* @__PURE__ */ jsxDEV("button", { id: "passphrase-btn", type: "submit", children: "unlock" })
        ] }),
        /* @__PURE__ */ jsxDEV("span", { id: "auth-status", style: "display:none" })
      ] }),
      /* @__PURE__ */ jsxDEV("div", { class: "accordion", children: games.map(({ key, state, ui }) => /* @__PURE__ */ jsxDEV(
        AccordionRow,
        {
          game: key,
          displayName: ui.displayName,
          state,
          connectAddress: ui.connectAddress,
          clientDownloadUrl: ui.clientDownloadUrl,
          startBlocked: ui.startBlocked,
          logsEnabled: ui.logsEnabled,
          logMode: ui.logMode,
          logUrl: ui.logUrl
        },
        key
      )) }),
      /* @__PURE__ */ jsxDEV("script", { src: "https://unpkg.com/htmx.org@2/dist/htmx.min.js" }),
      /* @__PURE__ */ jsxDEV("script", { src: "https://unpkg.com/htmx-ext-sse@2/sse.js" }),
      /* @__PURE__ */ jsxDEV("script", { dangerouslySetInnerHTML: { __html: initScript } })
    ] })
  ] });
  return "<!DOCTYPE html>" + page.toString();
}

// src/app.ts
var WEB_UI_PASSPHRASE = process.env.WEB_UI_PASSPHRASE ?? "";
var API_TOKEN = process.env.API_TOKEN ?? "";
var SIDECAR_TOKEN3 = process.env.SIDECAR_TOKEN ?? "";
var PUBLIC_HOST = process.env.PUBLIC_HOST ?? "localhost";
var BACKEND = process.env.BACKEND ?? "ecs";
var ENABLE_LOG_STREAMS = (process.env.ENABLE_LOG_STREAMS ?? "1") === "1";
var REGION2 = process.env.AWS_REGION ?? "ca-central-1";
var SIDECAR_HOST2 = process.env.SIDECAR_HOST ?? "localhost";
var cloudwatchLogs = new import_client_cloudwatch_logs.CloudWatchLogsClient({ region: REGION2 });
function statusFragment(state) {
  const ip = state.publicIp ? ` \u2014 ${state.publicIp}` : "";
  const players = state.players ? ` (${state.players} players)` : "";
  return `<span class="status ${state.status}">${state.status}${ip}${players}</span>`;
}
function gameUiConfig(gameKey, config, state, startBlocked) {
  const c = config;
  const logMode = BACKEND === "ecs" ? "poll" : "sse";
  return {
    displayName: c.displayName ?? null,
    connectAddress: c.connectPort ? `${PUBLIC_HOST}:${c.connectPort}` : null,
    clientDownloadUrl: c.clientDownloadUrl ?? null,
    startBlocked,
    logsEnabled: ENABLE_LOG_STREAMS,
    logMode,
    logUrl: `/logs?game=${encodeURIComponent(gameKey)}`
  };
}
function splitLogMessages(events) {
  const lines = [];
  for (const event of events) {
    const message = event.message ?? "";
    for (const line of message.split("\n")) {
      if (line !== "") lines.push(line);
    }
  }
  return lines;
}
function occupiedHostPorts(games, cache2) {
  const occupied = /* @__PURE__ */ new Set();
  for (const [key, config] of Object.entries(games)) {
    const state = cache2.get(key);
    if (!state || state.status === "offline") continue;
    const ports = config.ports ?? {};
    for (const binding of Object.values(ports)) {
      occupied.add(binding.hostPort);
    }
  }
  return occupied;
}
function hasPortConflict(config, occupied, ownKey, games, cache2) {
  const ownState = cache2.get(ownKey);
  if (ownState && ownState.status !== "offline") return false;
  const ports = config.ports ?? {};
  for (const binding of Object.values(ports)) {
    if (occupied.has(binding.hostPort)) return true;
  }
  return false;
}
function createApp(backend2, cache2) {
  const app2 = new Hono2();
  app2.use("*", async (c, next) => {
    const startedAt = Date.now();
    const method = c.req.method;
    const path2 = c.req.path;
    const query = c.req.query();
    const game = query.game ? ` game=${query.game}` : "";
    const operation = query.operation ? ` op=${query.operation}` : "";
    try {
      await next();
    } catch (error) {
      log.error(`http: ${method} ${path2}${game}${operation} failed`, error);
      throw error;
    }
    const durationMs = Date.now() - startedAt;
    const status = c.res.status;
    const base = `http: ${method} ${path2}${game}${operation} -> ${status} (${durationMs}ms)`;
    if (status >= 500) {
      log.error(base);
    } else if (status >= 400) {
      log.warn(base);
    } else {
      log.info(base);
    }
  });
  app2.get("/", async (c) => {
    const passphrase = c.req.header("x-passphrase") ?? "";
    const game = c.req.query("game");
    const operation = c.req.query("operation");
    if (game && operation) {
      if (passphrase !== WEB_UI_PASSPHRASE) {
        log.warn(`web: auth failure from ${c.req.header("x-forwarded-for") ?? "unknown"}`);
        return c.text("unauthorized", 401);
      }
      const games2 = backend2.getGames();
      const config = games2[game];
      if (!config) return c.html(`<span class="status">unknown game: ${game}</span>`, 400);
      let state;
      if (operation === "start") {
        log.info(`web: start ${game}`);
        state = await backend2.startGame(config);
        log.info(`web: start ${game} \u2192 ${state.status}`);
        cache2.set(game, await backend2.getCachedState(config));
      } else if (operation === "stop") {
        log.info(`web: stop ${game}`);
        state = await backend2.stopGame(config);
        log.info(`web: stop ${game} \u2192 ${state.status}`);
        cache2.set(game, await backend2.getCachedState(config));
      } else {
        state = await backend2.getGameState(config);
      }
      return c.html(statusFragment(state));
    }
    if (c.req.header("hx-request") && WEB_UI_PASSPHRASE !== "") {
      if (passphrase !== WEB_UI_PASSPHRASE) return c.text("unauthorized", 401);
      return c.text("ok");
    }
    await cache2.refreshIfStale();
    const games = backend2.getGames();
    const occupied = occupiedHostPorts(games, cache2);
    const rows = Object.entries(games).map(([key, config]) => {
      const state = cache2.get(key) ?? { status: "offline", players: 0, hostname: "", map: "", updatedAt: /* @__PURE__ */ new Date() };
      return {
        key,
        state,
        ui: gameUiConfig(key, config, state, hasPortConflict(config, occupied, key, games, cache2))
      };
    });
    return c.html(renderUi(rows));
  });
  app2.post("/", async (c) => {
    const passphrase = c.req.header("x-passphrase") ?? "";
    if (passphrase !== WEB_UI_PASSPHRASE) {
      log.warn(`web: auth failure from ${c.req.header("x-forwarded-for") ?? "unknown"}`);
      const isHtmx2 = !!c.req.header("hx-request");
      if (isHtmx2) return c.html(`<span class="status">unauthorized</span>`, 401);
      return c.json({ error: "unauthorized" }, 401);
    }
    const game = c.req.query("game");
    const opFromQuery = c.req.query("operation");
    const isHtmx = !!c.req.header("hx-request");
    let gameKey;
    let operation;
    if (game && opFromQuery) {
      gameKey = game;
      operation = opFromQuery;
    } else {
      const body = await c.req.json();
      gameKey = body.game;
      operation = body.operation;
    }
    const games = backend2.getGames();
    const config = games[gameKey];
    if (!config) {
      if (isHtmx) return c.html(`<span class="status">unknown game: ${gameKey}</span>`, 400);
      return c.json({ error: `unknown game: ${gameKey}` }, 400);
    }
    let state;
    if (operation === "start") {
      log.info(`web: start ${gameKey}`);
      state = await backend2.startGame(config);
      log.info(`web: start ${gameKey} \u2192 ${state.status}`);
      cache2.set(gameKey, await backend2.getCachedState(config));
    } else if (operation === "stop") {
      log.info(`web: stop ${gameKey}`);
      state = await backend2.stopGame(config);
      log.info(`web: stop ${gameKey} \u2192 ${state.status}`);
      cache2.set(gameKey, await backend2.getCachedState(config));
    } else {
      state = await backend2.getGameState(config);
    }
    if (isHtmx) return c.html(statusFragment(state));
    return c.json(state);
  });
  app2.get("/status", async (c) => {
    await cache2.refreshIfStale();
    const games = backend2.getGames();
    const states = Object.fromEntries(
      Object.keys(games).map((game) => [
        game,
        cache2.get(game) ?? { status: "offline", players: 0, hostname: "", map: "", updatedAt: /* @__PURE__ */ new Date() }
      ])
    );
    return c.json(states);
  });
  app2.get("/logs", async (c) => {
    if (!ENABLE_LOG_STREAMS) return c.text("log streaming disabled for this deployment", 503);
    const token = c.req.query("token") ?? "";
    if (token !== WEB_UI_PASSPHRASE) return c.text("unauthorized", 401);
    const game = c.req.query("game") ?? "";
    const games = backend2.getGames();
    const config = games[game];
    if (!config) return c.text(`unknown game: ${game}`, 400);
    if (BACKEND === "ecs") {
      const ecsConfig = config;
      if (!ecsConfig.logGroupName) return c.json({ error: "log group not configured" }, 503);
      const cursor = c.req.query("cursor") ?? void 0;
      const res = await cloudwatchLogs.send(new import_client_cloudwatch_logs.FilterLogEventsCommand({
        logGroupName: ecsConfig.logGroupName,
        nextToken: cursor,
        limit: 100,
        startTime: cursor ? void 0 : Date.now() - 5 * 60 * 1e3
      }));
      return c.json({
        lines: splitLogMessages(res.events ?? []),
        cursor: res.nextToken ?? cursor ?? null
      });
    }
    const cached = cache2.get(game);
    if (!cached || cached.status === "offline") return c.text("game offline", 503);
    const sidecarUrl = `http://${SIDECAR_HOST2}:${config.sidecarPort}/logs`;
    return streamSSE(c, async (stream2) => {
      log.info(`logs: stream opened for ${game}`);
      await stream2.writeSSE({ data: `<div class="log-line">[connecting to ${game} logs]</div>`, event: "log" });
      let res;
      try {
        res = await fetch(sidecarUrl, {
          headers: { Authorization: `Bearer ${SIDECAR_TOKEN3}` },
          signal: c.req.raw.signal
        });
      } catch (error) {
        log.info(`logs: stream closed for ${game} (connection error)`);
        await stream2.writeSSE({ data: `<div class="log-line">[log proxy error: ${error instanceof Error ? error.message : String(error)}]</div>`, event: "log" });
        await stream2.close();
        return;
      }
      if (!res.ok || !res.body) {
        log.warn(`logs: sidecar returned ${res.status} for ${game}`);
        await stream2.writeSSE({ data: `<div class="log-line">[log proxy error: sidecar returned HTTP ${res.status}]</div>`, event: "log" });
        await stream2.close();
        return;
      }
      await stream2.writeSSE({ data: `<div class="log-line">[connected to ${game} logs]</div>`, event: "log" });
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      let buf = "";
      try {
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          buf += decoder.decode(value, { stream: true });
          const lines = buf.split("\n");
          buf = lines.pop() ?? "";
          for (const line of lines) {
            if (line.startsWith("data: ")) await stream2.writeSSE({ data: `<div class="log-line">${line.slice(6)}</div>`, event: "log" });
          }
        }
      } catch {
      }
      log.info(`logs: stream closed for ${game}`);
    });
  });
  app2.get("/api", async (c) => {
    const token = c.req.header("x-api-token") ?? "";
    if (token !== API_TOKEN) return c.json({ error: "unauthorized" }, 401);
    const game = c.req.query("game") ?? "";
    const operation = c.req.query("operation");
    const games = backend2.getGames();
    const config = games[game];
    if (!config) return c.json({ error: `unknown game: ${game}` }, 400);
    if (operation === "start") {
      log.info(`api: start ${game}`);
      const state = await backend2.startGame(config);
      log.info(`api: start ${game} \u2192 ${state.status}`);
      cache2.set(game, await backend2.getCachedState(config));
      return c.json(state);
    }
    if (operation === "stop") {
      log.info(`api: stop ${game}`);
      const state = await backend2.stopGame(config);
      log.info(`api: stop ${game} \u2192 ${state.status}`);
      cache2.set(game, await backend2.getCachedState(config));
      return c.json(state);
    }
    return c.json(await backend2.getGameState(config));
  });
  app2.post("/discord", makeDiscordHandler(backend2));
  return app2;
}

// src/index.ts
var backend = createBackend();
var cache = new GameCache(backend);
var app = createApp(backend, cache);
var handler = streamHandle(app);
// Annotate the CommonJS export names for ESM import in node:
0 && (module.exports = {
  handler
});
