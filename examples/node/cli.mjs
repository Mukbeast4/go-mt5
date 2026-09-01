#!/usr/bin/env node

// A complete, dependency-free Node.js client for the Rust mt5-bridge.
// The bridge protocol is documented in ../../rust/README.md and
// ../../rust/proto/bridge.proto. This example intentionally uses the
// built-in net module and a small protobuf codec so it can be copied into a
// service without a generated-code or npm dependency step.

import net from "node:net";

const PROTOCOL_VERSION = 1;
const HEADER_AFTER_LENGTH = 20;
const MAX_FRAME_LENGTH = 1024 * 1024;
const MAX_METADATA_LENGTH = 64 * 1024;
const RATE_RECORD_BYTES = 60;
const TICK_RECORD_BYTES = 60;
const UINT32_MAX = 0xffff_ffffn;
const UINT64_MAX = 0xffff_ffff_ffff_ffffn;
const INT64_MIN = -(1n << 63n);
const INT64_MAX = (1n << 63n) - 1n;
const MAX_SAFE_BIGINT = BigInt(Number.MAX_SAFE_INTEGER);

const MESSAGE = Object.freeze({
  Hello: 1,
  HelloAck: 2,
  Request: 3,
  Response: 4,
  Error: 5,
  ResponseStart: 6,
  ResponseChunk: 7,
  ResponseEnd: 8,
  Cancel: 9,
  WindowUpdate: 10,
  Ping: 11,
  Pong: 12,
});

const OPERATION = Object.freeze({
  SymbolInfoTick: 8,
  CopyRatesFrom: 10,
  CopyRatesFromPos: 11,
  CopyRatesRange: 12,
  CopyTicksFrom: 13,
  CopyTicksRange: 14,
});

const OPERATION_BY_COMMAND = new Map([
  ["symbolinfotick", { name: "SymbolInfoTick", code: OPERATION.SymbolInfoTick }],
  ["copyratesfrompos", { name: "CopyRatesFromPos", code: OPERATION.CopyRatesFromPos, schema: 2 }],
  ["copyratesfrom", { name: "CopyRatesFrom", code: OPERATION.CopyRatesFrom, schema: 2 }],
  ["copyratesrange", { name: "CopyRatesRange", code: OPERATION.CopyRatesRange, schema: 2 }],
  ["copyticksfrom", { name: "CopyTicksFrom", code: OPERATION.CopyTicksFrom, schema: 3 }],
  ["copyticksrange", { name: "CopyTicksRange", code: OPERATION.CopyTicksRange, schema: 3 }],
]);

const TIMEFRAME = new Map([
  ["M1", 1],
  ["M2", 2],
  ["M3", 3],
  ["M4", 4],
  ["M5", 5],
  ["M6", 6],
  ["M10", 10],
  ["M12", 12],
  ["M15", 15],
  ["M20", 20],
  ["M30", 30],
  ["H1", 16_385],
  ["H2", 16_386],
  ["H3", 16_387],
  ["H4", 16_388],
  ["H6", 16_390],
  ["H8", 16_392],
  ["H12", 16_396],
  ["D1", 16_408],
  ["W1", 32_769],
  ["MN1", 49_153],
]);

const TICK_FLAGS = Object.freeze({
  // MQL5's COPY_TICKS_ALL is -1. The bridge request Value is unsigned, so
  // it is sent as the uint32 representation 0xffffffff.
  all: UINT32_MAX,
  info: 1n,
  trade: 2n,
});

class CliError extends Error {
  constructor(message) {
    super(message);
    this.name = "CliError";
  }
}

class BridgeError extends Error {
  constructor(message, details = null) {
    super(message);
    this.name = "BridgeError";
    this.details = details;
  }
}

function joinBuffers(parts) {
  return Buffer.concat(parts.filter((part) => part && part.length > 0));
}

function encodeVarint(value) {
  let number = typeof value === "bigint" ? value : BigInt(value);
  if (number < 0n || number > UINT64_MAX) {
    throw new CliError(`protobuf varint is outside uint64: ${number}`);
  }
  const bytes = [];
  do {
    let byte = Number(number & 0x7fn);
    number >>= 7n;
    if (number !== 0n) byte |= 0x80;
    bytes.push(byte);
  } while (number !== 0n);
  return Buffer.from(bytes);
}

function encodeKey(fieldNumber, wireType) {
  return encodeVarint(BigInt(fieldNumber * 8 + wireType));
}

function fieldVarint(fieldNumber, value) {
  return joinBuffers([encodeKey(fieldNumber, 0), encodeVarint(value)]);
}

function fieldBytes(fieldNumber, value) {
  const bytes = Buffer.isBuffer(value) ? value : Buffer.from(value);
  return joinBuffers([encodeKey(fieldNumber, 2), encodeVarint(bytes.length), bytes]);
}

function fieldString(fieldNumber, value) {
  return fieldBytes(fieldNumber, Buffer.from(value, "utf8"));
}

function fieldMessage(fieldNumber, message) {
  return fieldBytes(fieldNumber, message);
}

function encodeDouble(value) {
  if (!Number.isFinite(value)) throw new CliError(`expected a finite number, got ${value}`);
  const bytes = Buffer.allocUnsafe(8);
  bytes.writeDoubleLE(value, 0);
  return bytes;
}

// A bridge Value is a protobuf oneof. These helpers return the complete
// encoded Value message, which makes nested ValueObject construction clear.
function valueBool(value) {
  return fieldVarint(1, value ? 1n : 0n);
}

function valueI64(value) {
  const number = parseBigInt(value, "int64");
  if (number < INT64_MIN || number > INT64_MAX) {
    throw new CliError(`int64 is outside range: ${number}`);
  }
  const zigzag = (number << 1n) ^ (number >> 63n);
  return fieldVarint(2, zigzag);
}

function valueU64(value) {
  return fieldVarint(3, parseUnsigned(value, "uint64", UINT64_MAX));
}

function valueF64(value) {
  return joinBuffers([encodeKey(4, 1), encodeDouble(value)]);
}

function valueString(value) {
  return fieldString(5, value);
}

function valueObject(fields) {
  const entries = Object.entries(fields).map(([name, value]) => {
    const valueField = joinBuffers([fieldString(1, name), fieldMessage(2, value)]);
    return fieldMessage(1, valueField);
  });
  return fieldMessage(8, joinBuffers(entries));
}

function encodeHello(clientId, token) {
  return joinBuffers([fieldString(1, clientId), fieldBytes(2, Buffer.from(token, "utf8"))]);
}

function encodeRequest(operation, terminalEpoch, params, deadlineMs) {
  const fields = [
    fieldVarint(1, BigInt(operation)),
    fieldVarint(2, parseUnsigned(terminalEpoch, "terminal epoch", UINT64_MAX)),
  ];
  if (params) fields.push(fieldMessage(3, params));
  if (deadlineMs > 0) fields.push(fieldVarint(4, BigInt(deadlineMs)));
  return joinBuffers(fields);
}

function encodeWindowUpdate(creditBytes) {
  return fieldVarint(1, parseUnsigned(creditBytes, "response credit", UINT64_MAX));
}

function encodeCancel(reason) {
  return fieldString(1, reason);
}

function parseBigInt(value, name) {
  try {
    return typeof value === "bigint" ? value : BigInt(String(value));
  } catch {
    throw new CliError(`${name} must be an integer, got ${JSON.stringify(value)}`);
  }
}

function parseUnsigned(value, name, maximum = UINT64_MAX) {
  const number = parseBigInt(value, name);
  if (number < 0n || number > maximum) {
    throw new CliError(`${name} must be between 0 and ${maximum}, got ${number}`);
  }
  return number;
}

function readVarint(buffer, offset) {
  let value = 0n;
  for (let index = 0; index < 10; index += 1) {
    if (offset >= buffer.length) throw new BridgeError("truncated protobuf varint");
    const byte = buffer[offset++];
    if (index === 9 && byte > 1) throw new BridgeError("protobuf varint exceeds uint64");
    value |= BigInt(byte & 0x7f) << BigInt(index * 7);
    if ((byte & 0x80) === 0) return { value, offset };
  }
  throw new BridgeError("unterminated protobuf varint");
}

function parseProto(buffer) {
  const fields = [];
  let offset = 0;
  while (offset < buffer.length) {
    const key = readVarint(buffer, offset);
    offset = key.offset;
    const fieldNumberBig = key.value >> 3n;
    const wireType = Number(key.value & 7n);
    if (fieldNumberBig < 1n || fieldNumberBig > BigInt(Number.MAX_SAFE_INTEGER)) {
      throw new BridgeError("invalid protobuf field number");
    }
    const fieldNumber = Number(fieldNumberBig);
    let value;
    if (wireType === 0) {
      const result = readVarint(buffer, offset);
      value = result.value;
      offset = result.offset;
    } else if (wireType === 1) {
      if (offset + 8 > buffer.length) throw new BridgeError("truncated protobuf fixed64 field");
      value = buffer.subarray(offset, offset + 8);
      offset += 8;
    } else if (wireType === 2) {
      const length = readVarint(buffer, offset);
      offset = length.offset;
      if (length.value > BigInt(buffer.length - offset)) {
        throw new BridgeError("truncated protobuf bytes field");
      }
      const end = offset + Number(length.value);
      value = buffer.subarray(offset, end);
      offset = end;
    } else if (wireType === 5) {
      if (offset + 4 > buffer.length) throw new BridgeError("truncated protobuf fixed32 field");
      value = buffer.subarray(offset, offset + 4);
      offset += 4;
    } else {
      throw new BridgeError(`unsupported protobuf wire type ${wireType}`);
    }
    fields.push({ number: fieldNumber, wireType, value });
  }
  return fields;
}

function getField(fields, number) {
  for (let index = fields.length - 1; index >= 0; index -= 1) {
    if (fields[index].number === number) return fields[index];
  }
  return null;
}

function getFields(fields, number) {
  return fields.filter((field) => field.number === number);
}

function requireWire(field, wireType, name) {
  if (!field || field.wireType !== wireType) {
    throw new BridgeError(`protobuf field ${name} has an unexpected wire type`);
  }
  return field.value;
}

function optionalVarint(fields, number, fallback = 0n) {
  const field = getField(fields, number);
  return field ? requireWire(field, 0, number) : fallback;
}

function optionalBytes(fields, number, fallback = Buffer.alloc(0)) {
  const field = getField(fields, number);
  return field ? requireWire(field, 2, number) : fallback;
}

function optionalString(fields, number, fallback = "") {
  return optionalBytes(fields, number, null)?.toString("utf8") ?? fallback;
}

function signedInt64(value) {
  return value >= (1n << 63n) ? value - (1n << 64n) : value;
}

function decodeValue(buffer) {
  const fields = parseProto(buffer);
  const field = fields.length > 0 ? fields[fields.length - 1] : null;
  if (!field) return null;
  switch (field.number) {
    case 1:
      return Boolean(requireWire(field, 0, "bool_value"));
    case 2:
      return decodeZigzag(requireWire(field, 0, "i64_value"));
    case 3:
      return requireWire(field, 0, "u64_value");
    case 4:
      return requireWire(field, 1, "f64_value").readDoubleLE(0);
    case 5:
      return requireWire(field, 2, "string_value").toString("utf8");
    case 6:
      return requireWire(field, 2, "bytes_value");
    case 7: {
      const listFields = parseProto(requireWire(field, 2, "list_value"));
      return getFields(listFields, 1).map((item) => decodeValue(requireWire(item, 2, "list values")));
    }
    case 8: {
      const objectFields = parseProto(requireWire(field, 2, "object_value"));
      const result = Object.create(null);
      for (const entry of getFields(objectFields, 1)) {
        const entryFields = parseProto(requireWire(entry, 2, "object fields"));
        const name = requireWire(getField(entryFields, 1), 2, "field name").toString("utf8");
        const value = decodeValue(requireWire(getField(entryFields, 2), 2, "field value"));
        result[name] = value;
      }
      return result;
    }
    default:
      throw new BridgeError(`unknown bridge Value kind ${field.number}`);
  }
}

function decodeZigzag(value) {
  return (value >> 1n) ^ -(value & 1n);
}

function decodeHelloAck(buffer) {
  const fields = parseProto(buffer);
  return {
    bridgeInstanceId: optionalBytes(fields, 1),
    sessionId: optionalBytes(fields, 2),
    terminalEpoch: optionalVarint(fields, 3),
    terminalState: optionalString(fields, 4),
    terminalBuild: Number(optionalVarint(fields, 5)),
    accountLogin: signedInt64(optionalVarint(fields, 6)),
    accountServer: optionalString(fields, 7),
    maxFrameLength: Number(optionalVarint(fields, 8, BigInt(MAX_FRAME_LENGTH))),
    maxMetadataLength: Number(optionalVarint(fields, 9, BigInt(MAX_METADATA_LENGTH))),
    targetChunkBytes: Number(optionalVarint(fields, 10)),
    initialResponseCredit: optionalVarint(fields, 11),
    capabilities: optionalVarint(fields, 12),
  };
}

function decodeResponse(buffer) {
  const fields = parseProto(buffer);
  const resultField = getField(fields, 2);
  return {
    operation: Number(optionalVarint(fields, 1)),
    result: resultField ? decodeValue(requireWire(resultField, 2, "result")) : null,
  };
}

function decodeResponseStart(buffer) {
  const fields = parseProto(buffer);
  return {
    operation: Number(optionalVarint(fields, 1)),
    schema: Number(optionalVarint(fields, 2)),
    totalRowsKnown: Boolean(optionalVarint(fields, 3)),
    totalRows: optionalVarint(fields, 4),
  };
}

function decodeResponseChunk(buffer) {
  const fields = parseProto(buffer);
  return {
    sequence: optionalVarint(fields, 1),
    rowOffset: optionalVarint(fields, 2),
    rowCount: optionalVarint(fields, 3),
  };
}

function decodeError(buffer) {
  const fields = parseProto(buffer);
  return {
    origin: optionalString(fields, 1),
    code: optionalString(fields, 2),
    operation: optionalString(fields, 3),
    message: optionalString(fields, 4),
    nativeCode: signedInt64(optionalVarint(fields, 5)),
    certainty: Number(optionalVarint(fields, 6)),
  };
}

function decodeResponseEnd(buffer) {
  const fields = parseProto(buffer);
  const errorField = getField(fields, 4);
  return {
    success: Boolean(optionalVarint(fields, 1)),
    deliveredRows: optionalVarint(fields, 2),
    certainty: Number(optionalVarint(fields, 3)),
    error: errorField ? decodeError(requireWire(errorField, 2, "response error")) : null,
  };
}

function parseFrameBody(body) {
  if (body.length < HEADER_AFTER_LENGTH) throw new BridgeError("bridge frame is too short");
  const version = body.readUInt16LE(0);
  if (version !== PROTOCOL_VERSION) throw new BridgeError(`unsupported bridge protocol version ${version}`);
  const flags = body.readUInt32LE(4);
  if (flags !== 0) throw new BridgeError(`bridge frame has unsupported flags 0x${flags.toString(16)}`);
  const type = body.readUInt16LE(2);
  const requestId = body.readBigUInt64LE(8);
  const metadataLength = body.readUInt32LE(16);
  const payloadLength = body.length - HEADER_AFTER_LENGTH;
  if (metadataLength > MAX_METADATA_LENGTH || metadataLength > payloadLength) {
    throw new BridgeError(`invalid bridge metadata length ${metadataLength}`);
  }
  return {
    type,
    requestId,
    metadata: body.subarray(HEADER_AFTER_LENGTH, HEADER_AFTER_LENGTH + metadataLength),
    payload: body.subarray(HEADER_AFTER_LENGTH + metadataLength),
  };
}

class FrameConnection {
  constructor(socket) {
    this.socket = socket;
    this.buffer = Buffer.alloc(0);
    this.waiters = [];
    this.closedError = null;

    socket.on("data", (chunk) => {
      this.buffer = this.buffer.length === 0 ? chunk : Buffer.concat([this.buffer, chunk]);
      this.flushWaiters();
    });
    socket.on("error", (error) => this.fail(error));
    socket.on("end", () => this.fail(new BridgeError("bridge closed the connection")));
    socket.on("close", () => this.fail(new BridgeError("bridge connection closed")));
  }

  fail(error) {
    if (this.closedError) return;
    this.closedError = error;
    for (const waiter of this.waiters.splice(0)) waiter.reject(error);
  }

  flushWaiters() {
    while (this.waiters.length > 0 && this.buffer.length >= this.waiters[0].length) {
      const waiter = this.waiters.shift();
      const result = this.buffer.subarray(0, waiter.length);
      this.buffer = this.buffer.subarray(waiter.length);
      waiter.resolve(result);
    }
  }

  readExactly(length) {
    if (this.buffer.length >= length) {
      const result = this.buffer.subarray(0, length);
      this.buffer = this.buffer.subarray(length);
      return Promise.resolve(result);
    }
    if (this.closedError) return Promise.reject(this.closedError);
    return new Promise((resolve, reject) => {
      this.waiters.push({ length, resolve, reject });
      this.flushWaiters();
    });
  }

  async readFrame() {
    const lengthBytes = await this.readExactly(4);
    const frameLength = lengthBytes.readUInt32LE(0);
    if (frameLength < HEADER_AFTER_LENGTH || frameLength > MAX_FRAME_LENGTH) {
      throw new BridgeError(`invalid bridge frame length ${frameLength}`);
    }
    return parseFrameBody(await this.readExactly(frameLength));
  }

  writeFrame(type, requestId, metadata = Buffer.alloc(0), payload = Buffer.alloc(0)) {
    if (metadata.length > MAX_METADATA_LENGTH) throw new CliError("bridge metadata is too large");
    const frameLength = HEADER_AFTER_LENGTH + metadata.length + payload.length;
    if (frameLength > MAX_FRAME_LENGTH) throw new CliError("bridge frame is too large");
    if (this.closedError) return Promise.reject(this.closedError);

    const frame = Buffer.allocUnsafe(4 + frameLength);
    frame.writeUInt32LE(frameLength, 0);
    frame.writeUInt16LE(PROTOCOL_VERSION, 4);
    frame.writeUInt16LE(type, 6);
    frame.writeUInt32LE(0, 8);
    frame.writeBigUInt64LE(BigInt(requestId), 12);
    frame.writeUInt32LE(metadata.length, 20);
    metadata.copy(frame, 24);
    payload.copy(frame, 24 + metadata.length);

    return new Promise((resolve, reject) => {
      this.socket.write(frame, (error) => (error ? reject(error) : resolve()));
    });
  }

  close() {
    if (!this.closedError) this.fail(new BridgeError("bridge connection closed by client"));
    this.socket.destroy();
  }
}

function parseAddress(address) {
  const value = String(address);
  if (value.startsWith("[")) {
    const end = value.indexOf("]");
    if (end < 0 || value[end + 1] !== ":") throw new CliError(`address must be host:port, got ${value}`);
    return { host: value.slice(1, end), port: parsePort(value.slice(end + 2)) };
  }
  const separator = value.lastIndexOf(":");
  if (separator <= 0) throw new CliError(`address must be host:port, got ${value}`);
  return { host: value.slice(0, separator), port: parsePort(value.slice(separator + 1)) };
}

function parsePort(value) {
  const port = parseUnsigned(value, "port", 65_535n);
  if (port === 0n) throw new CliError("port must be greater than zero");
  return Number(port);
}

function withTimeout(promise, timeoutMs, onTimeout) {
  let timer;
  const timeout = new Promise((_, reject) => {
    timer = setTimeout(() => {
      onTimeout();
      reject(new BridgeError(`bridge operation timed out after ${timeoutMs}ms`));
    }, timeoutMs);
  });
  return Promise.race([promise, timeout]).finally(() => clearTimeout(timer));
}

class BridgeClient {
  constructor(connection, helloAck, timeoutMs) {
    this.connection = connection;
    this.helloAck = helloAck;
    this.timeoutMs = timeoutMs;
    this.nextRequestId = 1n;
  }

  static async connect({ address, token, clientId, timeoutMs }) {
    const endpoint = parseAddress(address);
    const socket = net.createConnection(endpoint);
    socket.setNoDelay(true);
    const connection = new FrameConnection(socket);
    try {
      await new Promise((resolve, reject) => {
        const timer = setTimeout(() => reject(new BridgeError(`could not connect within ${timeoutMs}ms`)), timeoutMs);
        socket.once("connect", () => {
          clearTimeout(timer);
          resolve();
        });
        socket.once("error", (error) => {
          clearTimeout(timer);
          reject(error);
        });
      });
      await connection.writeFrame(MESSAGE.Hello, 0n, encodeHello(clientId, token));
      const helloAckFrame = await withTimeout(
        connection.readFrame(),
        timeoutMs,
        () => connection.close(),
      );
      if (helloAckFrame.type !== MESSAGE.HelloAck || helloAckFrame.requestId !== 0n) {
        throw new BridgeError("expected HelloAck frame during handshake");
      }
      return new BridgeClient(connection, decodeHelloAck(helloAckFrame.metadata), timeoutMs);
    } catch (error) {
      connection.close();
      throw error;
    }
  }

  async request(operation, params) {
    const requestId = this.nextRequestId++;
    const request = encodeRequest(
      operation.code,
      this.helloAck.terminalEpoch,
      params,
      this.timeoutMs,
    );
    const exchange = this.exchange(requestId, operation, request);
    let timeoutId;
    let timedOut = false;
    const timeout = new Promise((_, reject) => {
      timeoutId = setTimeout(() => {
        timedOut = true;
        reject(new BridgeError(`operation ${operation.name} timed out after ${this.timeoutMs}ms`));
      }, this.timeoutMs + 1_000);
    });

    try {
      return await Promise.race([exchange, timeout]);
    } catch (error) {
      if (timedOut) {
        // Cancel is best effort. Destroying the connection also releases the
        // server-side request if the cancel cannot be delivered.
        this.connection.writeFrame(MESSAGE.Cancel, requestId, encodeCancel("client timeout"))
          .then(() => this.connection.close(), () => this.connection.close());
      }
      throw error;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  async exchange(requestId, operation, request) {
    await this.connection.writeFrame(MESSAGE.Request, requestId, request);
    const first = await this.connection.readFrame();
    if (first.requestId !== requestId) {
      throw new BridgeError(`response request id mismatch: expected ${requestId}, got ${first.requestId}`);
    }
    if (first.type === MESSAGE.Error) throw this.remoteError(first.metadata);
    if (first.type === MESSAGE.Response) {
      const response = decodeResponse(first.metadata);
      if (response.operation !== operation.code) throw new BridgeError("response operation mismatch");
      return response.result;
    }
    if (first.type !== MESSAGE.ResponseStart) {
      throw new BridgeError(`expected Response or ResponseStart, got message type ${first.type}`);
    }
    return this.readStream(requestId, operation, first);
  }

  async readStream(requestId, operation, firstFrame) {
    const start = decodeResponseStart(firstFrame.metadata);
    if (start.operation !== operation.code) throw new BridgeError("stream operation mismatch");
    if (start.schema !== operation.schema) throw new BridgeError(`unexpected response schema ${start.schema}`);

    const rows = [];
    let expectedSequence = 0n;
    let expectedOffset = 0n;
    for (;;) {
      const frame = await this.connection.readFrame();
      if (frame.requestId !== requestId) {
        throw new BridgeError(`stream request id mismatch: expected ${requestId}, got ${frame.requestId}`);
      }
      if (frame.type === MESSAGE.Error) throw this.remoteError(frame.metadata);
      if (frame.type === MESSAGE.ResponseChunk) {
        const chunk = decodeResponseChunk(frame.metadata);
        if (chunk.sequence !== expectedSequence || chunk.rowOffset !== expectedOffset) {
          throw new BridgeError("response chunks are out of order");
        }
        const rowCount = parseUnsigned(chunk.rowCount, "response row count", BigInt(Number.MAX_SAFE_INTEGER));
        const recordBytes = operation.schema === 2 ? RATE_RECORD_BYTES : TICK_RECORD_BYTES;
        if (frame.payload.length !== Number(rowCount) * recordBytes) {
          throw new BridgeError(`response chunk payload does not contain ${rowCount} records`);
        }
        rows.push(...decodeRawRecords(frame.payload, operation.schema, Number(rowCount)));
        expectedSequence += 1n;
        expectedOffset += rowCount;

        // The bridge starts each request with one MiB of response credit. A
        // credit update equal to the consumed frame keeps a long history
        // response flowing without requiring a fixed result-size limit.
        const consumed = BigInt(frame.metadata.length + frame.payload.length);
        if (consumed > 0n) {
          await this.connection.writeFrame(
            MESSAGE.WindowUpdate,
            requestId,
            encodeWindowUpdate(consumed),
          );
        }
        continue;
      }
      if (frame.type === MESSAGE.ResponseEnd) {
        const end = decodeResponseEnd(frame.metadata);
        if (!end.success) {
          const details = end.error ?? { message: "stream request failed" };
          throw new BridgeError(details.message, details);
        }
        if (end.deliveredRows !== BigInt(rows.length)) {
          throw new BridgeError(`bridge delivered ${end.deliveredRows} rows, decoded ${rows.length}`);
        }
        if (start.totalRowsKnown && start.totalRows !== end.deliveredRows) {
          throw new BridgeError(`bridge announced ${start.totalRows} rows, delivered ${end.deliveredRows}`);
        }
        return rows;
      }
      throw new BridgeError(`unexpected stream message type ${frame.type}`);
    }
  }

  remoteError(metadata) {
    const details = decodeError(metadata);
    return new BridgeError(`${details.code || "BridgeError"}: ${details.message || "request failed"}`, details);
  }

  close() {
    this.connection.close();
  }
}

function decodeRawRecords(payload, schema, count) {
  const records = [];
  for (let index = 0; index < count; index += 1) {
    const offset = index * 60;
    if (schema === 2) {
      records.push({
        time: payload.readBigInt64LE(offset),
        open: payload.readDoubleLE(offset + 8),
        high: payload.readDoubleLE(offset + 16),
        low: payload.readDoubleLE(offset + 24),
        close: payload.readDoubleLE(offset + 32),
        tick_volume: payload.readBigInt64LE(offset + 40),
        spread: payload.readInt32LE(offset + 48),
        real_volume: payload.readBigInt64LE(offset + 52),
      });
    } else {
      records.push({
        time: payload.readBigInt64LE(offset),
        bid: payload.readDoubleLE(offset + 8),
        ask: payload.readDoubleLE(offset + 16),
        last: payload.readDoubleLE(offset + 24),
        volume: payload.readBigUInt64LE(offset + 32),
        time_msc: payload.readBigInt64LE(offset + 40),
        flags: payload.readUInt32LE(offset + 48),
        volume_real: payload.readDoubleLE(offset + 52),
      });
    }
  }
  return records;
}

function canonicalOption(name) {
  const normalized = name.toLowerCase().replaceAll("-", "");
  return {
    addr: "address",
    client: "clientid",
    datefrom: "from",
    dateto: "to",
    startposition: "startpos",
    timeout: "timeoutms",
  }[normalized] ?? normalized;
}

function parseArguments(argv) {
  const options = Object.create(null);
  let command = null;
  for (let index = 0; index < argv.length; index += 1) {
    const argument = argv[index];
    if (argument === "-h" || argument === "--help") {
      options.help = true;
      continue;
    }
    if (argument.startsWith("--")) {
      const equals = argument.indexOf("=");
      const rawName = equals >= 0 ? argument.slice(2, equals) : argument.slice(2);
      const name = canonicalOption(rawName);
      const booleanOption = name === "pretty" || name === "verbose";
      let value = equals >= 0 ? argument.slice(equals + 1) : undefined;
      if (booleanOption) {
        options[name] = value === undefined ? true : value !== "false";
      } else {
        if (value === undefined) {
          value = argv[++index];
          if (value === undefined || value.startsWith("--")) {
            throw new CliError(`option --${rawName} needs a value`);
          }
        }
        options[name] = value;
      }
      continue;
    }
    if (command) throw new CliError(`unexpected argument ${argument}`);
    command = argument;
  }
  return { command, options };
}

function requiredOption(options, name) {
  const value = options[name];
  if (value === undefined || value === "") {
    const displayName = name.replaceAll("_", "-");
    throw new CliError(`--${displayName} is required`);
  }
  return value;
}

function parseTimeframe(value) {
  const key = String(value).toUpperCase();
  if (TIMEFRAME.has(key)) return BigInt(TIMEFRAME.get(key));
  return parseUnsigned(value, "timeframe", UINT32_MAX);
}

function parseCount(value) {
  return parseUnsigned(value, "count", UINT32_MAX);
}

function parseEpochSeconds(value, optionName) {
  const raw = String(value).trim();
  if (raw.toLowerCase() === "now") return BigInt(Math.floor(Date.now() / 1_000));
  if (/^[+-]?\d+$/.test(raw)) return parseBigInt(raw, optionName);
  const milliseconds = Date.parse(raw);
  if (!Number.isFinite(milliseconds)) {
    throw new CliError(`--${optionName} must be epoch seconds or an ISO-8601 date, got ${raw}`);
  }
  return BigInt(Math.floor(milliseconds / 1_000));
}

function parseFlags(value) {
  const raw = String(value).toLowerCase();
  if (Object.hasOwn(TICK_FLAGS, raw)) return TICK_FLAGS[raw];
  if (raw === "-1") return UINT32_MAX;
  return parseUnsigned(value, "flags", UINT32_MAX);
}

function buildOperation(command, options) {
  const normalized = String(command).toLowerCase().replaceAll("-", "").replaceAll("_", "");
  const operation = OPERATION_BY_COMMAND.get(normalized);
  if (!operation) throw new CliError(`unknown operation ${command}`);

  const fields = { symbol: valueString(requiredOption(options, "symbol")) };
  if (operation.name === "SymbolInfoTick") return { operation, params: valueObject(fields) };

  if (operation.name.startsWith("CopyRates")) {
    fields.timeframe = valueU64(parseTimeframe(requiredOption(options, "timeframe")));
    if (operation.name === "CopyRatesFromPos") {
      fields.start_pos = valueU64(parseUnsigned(requiredOption(options, "startpos"), "start-pos", UINT32_MAX));
      fields.count = valueU64(parseCount(requiredOption(options, "count")));
    } else if (operation.name === "CopyRatesFrom") {
      fields.date_from = valueI64(parseEpochSeconds(requiredOption(options, "from"), "from"));
      fields.count = valueU64(parseCount(requiredOption(options, "count")));
    } else {
      fields.date_from = valueI64(parseEpochSeconds(requiredOption(options, "from"), "from"));
      fields.date_to = valueI64(parseEpochSeconds(requiredOption(options, "to"), "to"));
    }
  } else {
    if (operation.name === "CopyTicksFrom") {
      fields.date_from = valueI64(parseEpochSeconds(requiredOption(options, "from"), "from"));
      fields.count = valueU64(parseCount(requiredOption(options, "count")));
    } else {
      fields.date_from = valueI64(parseEpochSeconds(requiredOption(options, "from"), "from"));
      fields.date_to = valueI64(parseEpochSeconds(requiredOption(options, "to"), "to"));
    }
    fields.flags = valueU64(parseFlags(options.flags ?? "all"));
  }
  return { operation, params: valueObject(fields) };
}

function parseTimeout(options) {
  const value = options.timeoutms ?? "30000";
  const timeout = parseUnsigned(value, "timeout-ms", 2_147_483_647n);
  if (timeout === 0n) throw new CliError("timeout-ms must be greater than zero");
  return Number(timeout);
}

function jsonSafe(value) {
  if (typeof value === "bigint") {
    return value >= -MAX_SAFE_BIGINT && value <= MAX_SAFE_BIGINT ? Number(value) : value.toString();
  }
  if (Buffer.isBuffer(value)) return value.toString("base64");
  if (Array.isArray(value)) return value.map(jsonSafe);
  if (value && typeof value === "object") {
    const result = {};
    for (const [key, item] of Object.entries(value)) result[key] = jsonSafe(item);
    return result;
  }
  return value;
}

function usage() {
  return `Usage:
  node examples/node/cli.mjs <operation> [options]

Operations:
  SymbolInfoTick       --symbol SYMBOL
  CopyRatesFromPos     --symbol SYMBOL --timeframe M1 --start-pos N --count N
  CopyRatesFrom        --symbol SYMBOL --timeframe M1 --from TIME --count N
  CopyRatesRange       --symbol SYMBOL --timeframe M1 --from TIME --to TIME
  CopyTicksFrom        --symbol SYMBOL --from TIME --count N [--flags all|info|trade]
  CopyTicksRange       --symbol SYMBOL --from TIME --to TIME [--flags all|info|trade]

Connection options:
  --address HOST:PORT   MT5 bridge address (default: MT5_BRIDGE_ADDR or 127.0.0.1:19550)
  --token TOKEN         Bridge token (default: MT5_BRIDGE_TOKEN)
  --client-id ID        Client identifier (default: node-mt5-example)
  --timeout-ms N        Request and connect timeout (default: 30000)
  --pretty              Pretty-print JSON
  --verbose             Log handshake details to stderr

TIME accepts broker epoch seconds or an ISO-8601 date. Numeric timestamps are
passed to MT5 unchanged. The bridge returns JSON with the operation name and
the decoded result. No npm dependencies are required.
`;
}

async function main() {
  const { command, options } = parseArguments(process.argv.slice(2));
  if (options.help || !command) {
    process.stdout.write(usage());
    return;
  }

  const timeoutMs = parseTimeout(options);
  const token = options.token ?? process.env.MT5_BRIDGE_TOKEN;
  if (!token) throw new CliError("set --token or MT5_BRIDGE_TOKEN");
  const address = options.address ?? process.env.MT5_BRIDGE_ADDR ?? "127.0.0.1:19550";
  const clientId = options.clientid ?? "node-mt5-example";
  const { operation, params } = buildOperation(command, options);

  let client;
  try {
    client = await BridgeClient.connect({ address, token, clientId, timeoutMs });
    if (options.verbose) {
      console.error(
        `Connected to ${address}: state=${client.helloAck.terminalState} ` +
        `build=${client.helloAck.terminalBuild} epoch=${client.helloAck.terminalEpoch}`,
      );
    }
    const result = await client.request(operation, params);
    process.stdout.write(`${JSON.stringify({ operation: operation.name, result: jsonSafe(result) }, null, options.pretty ? 2 : 0)}\n`);
  } finally {
    client?.close();
  }
}

main().catch((error) => {
  console.error(`mt5-bridge example: ${error.message}`);
  if (error instanceof BridgeError && error.details && process.argv.includes("--verbose")) {
    console.error(JSON.stringify(jsonSafe(error.details), null, 2));
  }
  process.exitCode = 1;
});
