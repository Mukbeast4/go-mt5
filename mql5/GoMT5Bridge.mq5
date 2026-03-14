#property copyright "go-mt5"
#property link      "https://github.com/mukbeast4/go-mt5"
#property version   "1.00"
#property strict

#include <JAson.mqh>

input int    InpPort        = 15555;
input string InpBindAddress = "127.0.0.1";

#define EA_VERSION "1.0.0"

int serverSocket = INVALID_HANDLE;
int clientSocket = INVALID_HANDLE;
string subscribedSymbols[];

int OnInit()
{
   serverSocket = SocketCreate();
   if(serverSocket == INVALID_HANDLE)
   {
      Print("[GoMT5Bridge] ERROR: SocketCreate failed: ", GetLastError());
      return INIT_FAILED;
   }

   if(!SocketBind(serverSocket, InpBindAddress, InpPort))
   {
      Print("[GoMT5Bridge] ERROR: SocketBind failed on ", InpBindAddress, ":", InpPort, " error: ", GetLastError());
      SocketClose(serverSocket);
      return INIT_FAILED;
   }

   if(!SocketListen(serverSocket, 1))
   {
      Print("[GoMT5Bridge] ERROR: SocketListen failed: ", GetLastError());
      SocketClose(serverSocket);
      return INIT_FAILED;
   }

   EventSetMillisecondTimer(50);
   Print("[GoMT5Bridge] Listening on ", InpBindAddress, ":", InpPort);
   return INIT_SUCCEEDED;
}

void OnDeinit(const int reason)
{
   EventKillTimer();
   if(clientSocket != INVALID_HANDLE) SocketClose(clientSocket);
   if(serverSocket != INVALID_HANDLE) SocketClose(serverSocket);
   Print("[GoMT5Bridge] Shutdown");
}

void OnTimer()
{
   if(clientSocket == INVALID_HANDLE)
   {
      clientSocket = SocketAccept(serverSocket, 0);
      if(clientSocket != INVALID_HANDLE)
         Print("[GoMT5Bridge] Client connected");
      return;
   }

   uchar buf[];
   int received = SocketRead(clientSocket, buf, 4, 0);
   if(received < 0)
   {
      int err = GetLastError();
      if(err == 4014) return;
      Print("[GoMT5Bridge] Client disconnected: ", err);
      SocketClose(clientSocket);
      clientSocket = INVALID_HANDLE;
      return;
   }

   if(received < 4) return;

   uint payloadSize = (uint)buf[0] << 24 | (uint)buf[1] << 16 | (uint)buf[2] << 8 | (uint)buf[3];
   if(payloadSize == 0 || payloadSize > 67108864) return;

   uchar payload[];
   int totalRead = 0;
   while(totalRead < (int)payloadSize)
   {
      uchar chunk[];
      int r = SocketRead(clientSocket, chunk, (int)payloadSize - totalRead, 5000);
      if(r <= 0) return;
      ArrayCopy(payload, chunk, totalRead);
      totalRead += r;
   }

   string jsonStr = CharArrayToString(payload, 0, totalRead, CP_UTF8);

   CJAVal request;
   if(!request.Deserialize(jsonStr))
   {
      Print("[GoMT5Bridge] Invalid JSON: ", jsonStr);
      return;
   }

   string id = request["id"].ToStr();
   string action = request["action"].ToStr();

   CJAVal response;
   response["id"].Set(id);
   response["type"].Set("response");

   if(action == "ping")                    HandlePing(request, response);
   else if(action == "version")            HandleVersion(request, response);
   else if(action == "account_info")       HandleAccountInfo(request, response);
   else if(action == "terminal_info")      HandleTerminalInfo(request, response);
   else if(action == "symbols_total")      HandleSymbolsTotal(request, response);
   else if(action == "symbols_get")        HandleSymbolsGet(request, response);
   else if(action == "symbol_info")        HandleSymbolInfo(request, response);
   else if(action == "symbol_info_tick")   HandleSymbolInfoTick(request, response);
   else if(action == "symbol_select")      HandleSymbolSelect(request, response);
   else if(action == "copy_rates_from")    HandleCopyRatesFrom(request, response);
   else if(action == "copy_rates_from_pos") HandleCopyRatesFromPos(request, response);
   else if(action == "copy_rates_range")   HandleCopyRatesRange(request, response);
   else if(action == "copy_ticks_from")    HandleCopyTicksFrom(request, response);
   else if(action == "copy_ticks_range")   HandleCopyTicksRange(request, response);
   else if(action == "market_book_add")    HandleMarketBookAdd(request, response);
   else if(action == "market_book_get")    HandleMarketBookGet(request, response);
   else if(action == "market_book_release") HandleMarketBookRelease(request, response);
   else if(action == "order_send")         HandleOrderSend(request, response);
   else if(action == "order_check")        HandleOrderCheck(request, response);
   else if(action == "order_calc_margin")  HandleOrderCalcMargin(request, response);
   else if(action == "order_calc_profit")  HandleOrderCalcProfit(request, response);
   else if(action == "orders_total")       HandleOrdersTotal(request, response);
   else if(action == "orders_get")         HandleOrdersGet(request, response);
   else if(action == "positions_total")    HandlePositionsTotal(request, response);
   else if(action == "positions_get")      HandlePositionsGet(request, response);
   else if(action == "history_orders_total") HandleHistoryOrdersTotal(request, response);
   else if(action == "history_orders_get")   HandleHistoryOrdersGet(request, response);
   else if(action == "history_deals_total")  HandleHistoryDealsTotal(request, response);
   else if(action == "history_deals_get")    HandleHistoryDealsGet(request, response);
   else if(action == "subscribe_ticks")      HandleSubscribeTicks(request, response);
   else if(action == "unsubscribe_ticks")    HandleUnsubscribeTicks(request, response);
   else
   {
      SetError(response, 10013, "Unknown action: " + action);
   }

   SendResponse(response);
}

void OnTick()
{
   if(clientSocket == INVALID_HANDLE) return;
   if(ArraySize(subscribedSymbols) == 0) return;

   string sym = Symbol();
   bool found = false;
   for(int i = 0; i < ArraySize(subscribedSymbols); i++)
   {
      if(subscribedSymbols[i] == sym) { found = true; break; }
   }
   if(!found) return;

   MqlTick tick;
   if(!SymbolInfoTick(sym, tick)) return;

   CJAVal ev;
   ev["type"].Set("event");
   ev["event"].Set("tick");
   ev["data"]["symbol"].Set(sym);
   ev["data"]["bid"].Set(tick.bid);
   ev["data"]["ask"].Set(tick.ask);
   ev["data"]["last"].Set(tick.last);
   ev["data"]["volume"].Set((long)tick.volume);
   ev["data"]["time"].Set((long)tick.time);
   ev["data"]["time_msc"].Set(tick.time_msc);
   ev["data"]["flags"].Set(tick.flags);

   SendResponse(ev);
}

void SendResponse(CJAVal &response)
{
   if(clientSocket == INVALID_HANDLE) return;

   string jsonResp = response.Serialize();
   uchar payload[];
   StringToCharArray(jsonResp, payload, 0, StringLen(jsonResp), CP_UTF8);
   int payloadLen = ArraySize(payload);

   uchar header[4];
   header[0] = (uchar)((payloadLen >> 24) & 0xFF);
   header[1] = (uchar)((payloadLen >> 16) & 0xFF);
   header[2] = (uchar)((payloadLen >> 8) & 0xFF);
   header[3] = (uchar)(payloadLen & 0xFF);

   SocketSend(clientSocket, header, 4);
   SocketSend(clientSocket, payload, payloadLen);
}

void SetError(CJAVal &response, int code, string message)
{
   response["success"].Set(false);
   response["error"]["code"].Set(code);
   response["error"]["message"].Set(message);
}

void SetSuccess(CJAVal &response)
{
   response["success"].Set(true);
}

void HandlePing(CJAVal &request, CJAVal &response)
{
   SetSuccess(response);
   response["data"].Set("pong");
}

void HandleVersion(CJAVal &request, CJAVal &response)
{
   SetSuccess(response);
   response["data"]["version"].Set(TerminalInfoString(TERMINAL_NAME));
   response["data"]["build"].Set(TerminalInfoInteger(TERMINAL_BUILD));
   response["data"]["build_date"].Set(TimeToString(TimeCurrent()));
   response["data"]["ea_version"].Set(EA_VERSION);
}

void HandleAccountInfo(CJAVal &request, CJAVal &response)
{
   SetSuccess(response);
   response["data"]["login"].Set(AccountInfoInteger(ACCOUNT_LOGIN));
   response["data"]["name"].Set(AccountInfoString(ACCOUNT_NAME));
   response["data"]["server"].Set(AccountInfoString(ACCOUNT_SERVER));
   response["data"]["currency"].Set(AccountInfoString(ACCOUNT_CURRENCY));
   response["data"]["balance"].Set(AccountInfoDouble(ACCOUNT_BALANCE));
   response["data"]["equity"].Set(AccountInfoDouble(ACCOUNT_EQUITY));
   response["data"]["margin"].Set(AccountInfoDouble(ACCOUNT_MARGIN));
   response["data"]["free_margin"].Set(AccountInfoDouble(ACCOUNT_MARGIN_FREE));
   response["data"]["margin_level"].Set(AccountInfoDouble(ACCOUNT_MARGIN_LEVEL));
   response["data"]["profit"].Set(AccountInfoDouble(ACCOUNT_PROFIT));
   response["data"]["leverage"].Set(AccountInfoInteger(ACCOUNT_LEVERAGE));
   response["data"]["trade_mode"].Set(AccountInfoInteger(ACCOUNT_TRADE_MODE));
   response["data"]["limit_orders"].Set(AccountInfoInteger(ACCOUNT_LIMIT_ORDERS));
   response["data"]["trade_allowed"].Set(AccountInfoInteger(ACCOUNT_TRADE_ALLOWED) != 0);
   response["data"]["trade_expert"].Set(AccountInfoInteger(ACCOUNT_TRADE_EXPERT) != 0);
   response["data"]["currency_digits"].Set(AccountInfoInteger(ACCOUNT_CURRENCY_DIGITS));
}

void HandleTerminalInfo(CJAVal &request, CJAVal &response)
{
   SetSuccess(response);
   response["data"]["build"].Set(TerminalInfoInteger(TERMINAL_BUILD));
   response["data"]["name"].Set(TerminalInfoString(TERMINAL_NAME));
   response["data"]["path"].Set(TerminalInfoString(TERMINAL_PATH));
   response["data"]["data_path"].Set(TerminalInfoString(TERMINAL_DATA_PATH));
   response["data"]["common_path"].Set(TerminalInfoString(TERMINAL_COMMONDATA_PATH));
   response["data"]["connected"].Set(TerminalInfoInteger(TERMINAL_CONNECTED) != 0);
   response["data"]["community_connection"].Set(TerminalInfoInteger(TERMINAL_COMMUNITY_CONNECTION) != 0);
   response["data"]["trade_allowed"].Set(TerminalInfoInteger(TERMINAL_TRADE_ALLOWED) != 0);
   response["data"]["dlls_allowed"].Set(TerminalInfoInteger(TERMINAL_DLLS_ALLOWED) != 0);
}

void HandleSymbolsTotal(CJAVal &request, CJAVal &response)
{
   SetSuccess(response);
   response["data"].Set(SymbolsTotal(false));
}

void HandleSymbolsGet(CJAVal &request, CJAVal &response)
{
   string group = request["params"]["group"].ToStr();
   int total = SymbolsTotal(false);
   SetSuccess(response);

   int idx = 0;
   for(int i = 0; i < total; i++)
   {
      string name = SymbolName(i, false);
      if(group != "" && StringFind(name, group) < 0) continue;

      response["data"][idx]["name"].Set(name);
      response["data"][idx]["bid"].Set(SymbolInfoDouble(name, SYMBOL_BID));
      response["data"][idx]["ask"].Set(SymbolInfoDouble(name, SYMBOL_ASK));
      response["data"][idx]["digits"].Set(SymbolInfoInteger(name, SYMBOL_DIGITS));
      response["data"][idx]["point"].Set(SymbolInfoDouble(name, SYMBOL_POINT));
      response["data"][idx]["spread"].Set(SymbolInfoInteger(name, SYMBOL_SPREAD));
      response["data"][idx]["visible"].Set(SymbolInfoInteger(name, SYMBOL_VISIBLE) != 0);
      idx++;
   }
}

void HandleSymbolInfo(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   if(!SymbolInfoInteger(symbol, SYMBOL_EXIST))
   {
      SetError(response, 10013, "Symbol not found: " + symbol);
      return;
   }

   SetSuccess(response);
   response["data"]["name"].Set(symbol);
   response["data"]["description"].Set(SymbolInfoString(symbol, SYMBOL_DESCRIPTION));
   response["data"]["path"].Set(SymbolInfoString(symbol, SYMBOL_PATH));
   response["data"]["currency_base"].Set(SymbolInfoString(symbol, SYMBOL_CURRENCY_BASE));
   response["data"]["currency_profit"].Set(SymbolInfoString(symbol, SYMBOL_CURRENCY_PROFIT));
   response["data"]["currency_margin"].Set(SymbolInfoString(symbol, SYMBOL_CURRENCY_MARGIN));
   response["data"]["bid"].Set(SymbolInfoDouble(symbol, SYMBOL_BID));
   response["data"]["ask"].Set(SymbolInfoDouble(symbol, SYMBOL_ASK));
   response["data"]["last"].Set(SymbolInfoDouble(symbol, SYMBOL_LAST));
   response["data"]["volume"].Set(SymbolInfoInteger(symbol, SYMBOL_VOLUME));
   response["data"]["volume_min"].Set(SymbolInfoDouble(symbol, SYMBOL_VOLUME_MIN));
   response["data"]["volume_max"].Set(SymbolInfoDouble(symbol, SYMBOL_VOLUME_MAX));
   response["data"]["volume_step"].Set(SymbolInfoDouble(symbol, SYMBOL_VOLUME_STEP));
   response["data"]["point"].Set(SymbolInfoDouble(symbol, SYMBOL_POINT));
   response["data"]["digits"].Set(SymbolInfoInteger(symbol, SYMBOL_DIGITS));
   response["data"]["spread"].Set(SymbolInfoInteger(symbol, SYMBOL_SPREAD));
   response["data"]["spread_float"].Set(SymbolInfoInteger(symbol, SYMBOL_SPREAD_FLOAT) != 0);
   response["data"]["trade_mode"].Set(SymbolInfoInteger(symbol, SYMBOL_TRADE_MODE));
   response["data"]["trade_calc_mode"].Set(SymbolInfoInteger(symbol, SYMBOL_TRADE_CALC_MODE));
   response["data"]["contract_size"].Set(SymbolInfoDouble(symbol, SYMBOL_TRADE_CONTRACT_SIZE));
   response["data"]["tick_value"].Set(SymbolInfoDouble(symbol, SYMBOL_TRADE_TICK_VALUE));
   response["data"]["tick_size"].Set(SymbolInfoDouble(symbol, SYMBOL_TRADE_TICK_SIZE));
   response["data"]["swap_long"].Set(SymbolInfoDouble(symbol, SYMBOL_SWAP_LONG));
   response["data"]["swap_short"].Set(SymbolInfoDouble(symbol, SYMBOL_SWAP_SHORT));
   response["data"]["visible"].Set(SymbolInfoInteger(symbol, SYMBOL_VISIBLE) != 0);
   response["data"]["session_deals"].Set(SymbolInfoInteger(symbol, SYMBOL_SESSION_DEALS));
   response["data"]["session_buy_orders"].Set(SymbolInfoInteger(symbol, SYMBOL_SESSION_BUY_ORDERS));
   response["data"]["session_sell_orders"].Set(SymbolInfoInteger(symbol, SYMBOL_SESSION_SELL_ORDERS));
}

void HandleSymbolInfoTick(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   MqlTick tick;
   if(!SymbolInfoTick(symbol, tick))
   {
      SetError(response, 10013, "Failed to get tick for: " + symbol);
      return;
   }

   SetSuccess(response);
   response["data"]["time"].Set((long)tick.time);
   response["data"]["time_msc"].Set(tick.time_msc);
   response["data"]["bid"].Set(tick.bid);
   response["data"]["ask"].Set(tick.ask);
   response["data"]["last"].Set(tick.last);
   response["data"]["volume"].Set((long)tick.volume);
   response["data"]["flags"].Set(tick.flags);
}

void HandleSymbolSelect(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   bool enable = request["params"]["enable"].ToBool();

   if(!SymbolSelect(symbol, enable))
   {
      SetError(response, 10013, "Failed to select symbol: " + symbol);
      return;
   }
   SetSuccess(response);
}

void HandleCopyRatesFrom(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   ENUM_TIMEFRAMES tf = (ENUM_TIMEFRAMES)request["params"]["timeframe"].ToInt();
   datetime dateFrom = (datetime)request["params"]["date_from"].ToInt();
   int count = request["params"]["count"].ToInt();

   MqlRates rates[];
   int copied = CopyRates(symbol, tf, dateFrom, count, rates);
   if(copied < 0)
   {
      SetError(response, GetLastError(), "CopyRates failed");
      return;
   }

   SetSuccess(response);
   for(int i = 0; i < copied; i++)
   {
      response["data"][i]["time"].Set((long)rates[i].time);
      response["data"][i]["open"].Set(rates[i].open);
      response["data"][i]["high"].Set(rates[i].high);
      response["data"][i]["low"].Set(rates[i].low);
      response["data"][i]["close"].Set(rates[i].close);
      response["data"][i]["tick_volume"].Set(rates[i].tick_volume);
      response["data"][i]["spread"].Set(rates[i].spread);
      response["data"][i]["real_volume"].Set(rates[i].real_volume);
   }
}

void HandleCopyRatesFromPos(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   ENUM_TIMEFRAMES tf = (ENUM_TIMEFRAMES)request["params"]["timeframe"].ToInt();
   int startPos = request["params"]["start_pos"].ToInt();
   int count = request["params"]["count"].ToInt();

   MqlRates rates[];
   int copied = CopyRates(symbol, tf, startPos, count, rates);
   if(copied < 0)
   {
      SetError(response, GetLastError(), "CopyRates failed");
      return;
   }

   SetSuccess(response);
   for(int i = 0; i < copied; i++)
   {
      response["data"][i]["time"].Set((long)rates[i].time);
      response["data"][i]["open"].Set(rates[i].open);
      response["data"][i]["high"].Set(rates[i].high);
      response["data"][i]["low"].Set(rates[i].low);
      response["data"][i]["close"].Set(rates[i].close);
      response["data"][i]["tick_volume"].Set(rates[i].tick_volume);
      response["data"][i]["spread"].Set(rates[i].spread);
      response["data"][i]["real_volume"].Set(rates[i].real_volume);
   }
}

void HandleCopyRatesRange(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   ENUM_TIMEFRAMES tf = (ENUM_TIMEFRAMES)request["params"]["timeframe"].ToInt();
   datetime dateFrom = (datetime)request["params"]["date_from"].ToInt();
   datetime dateTo = (datetime)request["params"]["date_to"].ToInt();

   MqlRates rates[];
   int copied = CopyRates(symbol, tf, dateFrom, dateTo, rates);
   if(copied < 0)
   {
      SetError(response, GetLastError(), "CopyRates failed");
      return;
   }

   SetSuccess(response);
   for(int i = 0; i < copied; i++)
   {
      response["data"][i]["time"].Set((long)rates[i].time);
      response["data"][i]["open"].Set(rates[i].open);
      response["data"][i]["high"].Set(rates[i].high);
      response["data"][i]["low"].Set(rates[i].low);
      response["data"][i]["close"].Set(rates[i].close);
      response["data"][i]["tick_volume"].Set(rates[i].tick_volume);
      response["data"][i]["spread"].Set(rates[i].spread);
      response["data"][i]["real_volume"].Set(rates[i].real_volume);
   }
}

void HandleCopyTicksFrom(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   datetime dateFrom = (datetime)request["params"]["date_from"].ToInt();
   int count = request["params"]["count"].ToInt();
   uint flags = (uint)request["params"]["flags"].ToInt();

   MqlTick ticks[];
   int copied = CopyTicks(symbol, ticks, COPY_TICKS_ALL, (ulong)dateFrom * 1000, count);
   if(copied < 0)
   {
      SetError(response, GetLastError(), "CopyTicks failed");
      return;
   }

   SetSuccess(response);
   for(int i = 0; i < copied; i++)
   {
      response["data"][i]["time"].Set((long)ticks[i].time);
      response["data"][i]["time_msc"].Set(ticks[i].time_msc);
      response["data"][i]["bid"].Set(ticks[i].bid);
      response["data"][i]["ask"].Set(ticks[i].ask);
      response["data"][i]["last"].Set(ticks[i].last);
      response["data"][i]["volume"].Set((long)ticks[i].volume);
      response["data"][i]["flags"].Set(ticks[i].flags);
   }
}

void HandleCopyTicksRange(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   datetime dateFrom = (datetime)request["params"]["date_from"].ToInt();
   datetime dateTo = (datetime)request["params"]["date_to"].ToInt();
   uint flags = (uint)request["params"]["flags"].ToInt();

   MqlTick ticks[];
   int copied = CopyTicksRange(symbol, ticks, COPY_TICKS_ALL, (ulong)dateFrom * 1000, (ulong)dateTo * 1000);
   if(copied < 0)
   {
      SetError(response, GetLastError(), "CopyTicksRange failed");
      return;
   }

   SetSuccess(response);
   for(int i = 0; i < copied; i++)
   {
      response["data"][i]["time"].Set((long)ticks[i].time);
      response["data"][i]["time_msc"].Set(ticks[i].time_msc);
      response["data"][i]["bid"].Set(ticks[i].bid);
      response["data"][i]["ask"].Set(ticks[i].ask);
      response["data"][i]["last"].Set(ticks[i].last);
      response["data"][i]["volume"].Set((long)ticks[i].volume);
      response["data"][i]["flags"].Set(ticks[i].flags);
   }
}

void HandleMarketBookAdd(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   if(!MarketBookAdd(symbol))
   {
      SetError(response, GetLastError(), "MarketBookAdd failed");
      return;
   }
   SetSuccess(response);
}

void HandleMarketBookGet(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   MqlBookInfo book[];
   if(!MarketBookGet(symbol, book))
   {
      SetError(response, GetLastError(), "MarketBookGet failed");
      return;
   }

   SetSuccess(response);
   for(int i = 0; i < ArraySize(book); i++)
   {
      response["data"][i]["type"].Set(book[i].type);
      response["data"][i]["price"].Set(book[i].price);
      response["data"][i]["volume"].Set(book[i].volume);
      response["data"][i]["volume_real"].Set(book[i].volume_real);
   }
}

void HandleMarketBookRelease(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   if(!MarketBookRelease(symbol))
   {
      SetError(response, GetLastError(), "MarketBookRelease failed");
      return;
   }
   SetSuccess(response);
}

void HandleOrderSend(CJAVal &request, CJAVal &response)
{
   MqlTradeRequest req;
   MqlTradeResult  res;

   req.action      = (ENUM_TRADE_REQUEST_ACTIONS)request["params"]["action"].ToInt();
   req.magic       = request["params"]["magic"].ToInt();
   req.order       = request["params"]["order"].ToInt();
   req.symbol      = request["params"]["symbol"].ToStr();
   req.volume      = request["params"]["volume"].ToDbl();
   req.price       = request["params"]["price"].ToDbl();
   req.stoplimit   = request["params"]["stoplimit"].ToDbl();
   req.sl          = request["params"]["sl"].ToDbl();
   req.tp          = request["params"]["tp"].ToDbl();
   req.deviation   = (ulong)request["params"]["deviation"].ToInt();
   req.type        = (ENUM_ORDER_TYPE)request["params"]["type"].ToInt();
   req.type_filling = (ENUM_ORDER_TYPE_FILLING)request["params"]["type_filling"].ToInt();
   req.type_time   = (ENUM_ORDER_TYPE_TIME)request["params"]["type_time"].ToInt();
   req.expiration  = (datetime)request["params"]["expiration"].ToInt();
   req.comment     = request["params"]["comment"].ToStr();
   req.position    = request["params"]["position"].ToInt();
   req.position_by = request["params"]["position_by"].ToInt();

   if(!OrderSend(req, res))
   {
      SetError(response, (int)res.retcode, res.comment);
      return;
   }

   SetSuccess(response);
   response["data"]["retcode"].Set((int)res.retcode);
   response["data"]["deal"].Set(res.deal);
   response["data"]["order"].Set(res.order);
   response["data"]["volume"].Set(res.volume);
   response["data"]["price"].Set(res.price);
   response["data"]["bid"].Set(res.bid);
   response["data"]["ask"].Set(res.ask);
   response["data"]["comment"].Set(res.comment);
   response["data"]["request_id"].Set((int)res.request_id);
   response["data"]["retcode_external"].Set(res.retcode_external);
}

void HandleOrderCheck(CJAVal &request, CJAVal &response)
{
   MqlTradeRequest req;
   MqlTradeCheckResult res;

   req.action      = (ENUM_TRADE_REQUEST_ACTIONS)request["params"]["action"].ToInt();
   req.symbol      = request["params"]["symbol"].ToStr();
   req.volume      = request["params"]["volume"].ToDbl();
   req.price       = request["params"]["price"].ToDbl();
   req.sl          = request["params"]["sl"].ToDbl();
   req.tp          = request["params"]["tp"].ToDbl();
   req.deviation   = (ulong)request["params"]["deviation"].ToInt();
   req.type        = (ENUM_ORDER_TYPE)request["params"]["type"].ToInt();
   req.type_filling = (ENUM_ORDER_TYPE_FILLING)request["params"]["type_filling"].ToInt();

   if(!OrderCheck(req, res))
   {
      SetError(response, (int)res.retcode, res.comment);
      return;
   }

   SetSuccess(response);
   response["data"]["retcode"].Set((int)res.retcode);
   response["data"]["balance"].Set(res.balance);
   response["data"]["equity"].Set(res.equity);
   response["data"]["profit"].Set(res.profit);
   response["data"]["margin"].Set(res.margin);
   response["data"]["margin_free"].Set(res.margin_free);
   response["data"]["margin_level"].Set(res.margin_level);
   response["data"]["comment"].Set(res.comment);
}

void HandleOrderCalcMargin(CJAVal &request, CJAVal &response)
{
   ENUM_ORDER_TYPE action = (ENUM_ORDER_TYPE)request["params"]["action"].ToInt();
   string symbol = request["params"]["symbol"].ToStr();
   double volume = request["params"]["volume"].ToDbl();
   double price = request["params"]["price"].ToDbl();
   double margin;

   if(!OrderCalcMargin(action, symbol, volume, price, margin))
   {
      SetError(response, GetLastError(), "OrderCalcMargin failed");
      return;
   }

   SetSuccess(response);
   response["data"].Set(margin);
}

void HandleOrderCalcProfit(CJAVal &request, CJAVal &response)
{
   ENUM_ORDER_TYPE action = (ENUM_ORDER_TYPE)request["params"]["action"].ToInt();
   string symbol = request["params"]["symbol"].ToStr();
   double volume = request["params"]["volume"].ToDbl();
   double priceOpen = request["params"]["price_open"].ToDbl();
   double priceClose = request["params"]["price_close"].ToDbl();
   double profit;

   if(!OrderCalcProfit(action, symbol, volume, priceOpen, priceClose, profit))
   {
      SetError(response, GetLastError(), "OrderCalcProfit failed");
      return;
   }

   SetSuccess(response);
   response["data"].Set(profit);
}

void HandleOrdersTotal(CJAVal &request, CJAVal &response)
{
   SetSuccess(response);
   response["data"].Set(OrdersTotal());
}

void HandleOrdersGet(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   long ticket = request["params"]["ticket"].ToInt();

   SetSuccess(response);
   int total = OrdersTotal();
   int idx = 0;

   for(int i = 0; i < total; i++)
   {
      ulong orderTicket = OrderGetTicket(i);
      if(orderTicket == 0) continue;

      if(symbol != "" && OrderGetString(ORDER_SYMBOL) != symbol) continue;
      if(ticket > 0 && (long)orderTicket != ticket) continue;

      response["data"][idx]["ticket"].Set(orderTicket);
      response["data"][idx]["time_setup"].Set(OrderGetInteger(ORDER_TIME_SETUP));
      response["data"][idx]["time_setup_msc"].Set(OrderGetInteger(ORDER_TIME_SETUP_MSC));
      response["data"][idx]["time_done"].Set(OrderGetInteger(ORDER_TIME_DONE));
      response["data"][idx]["time_done_msc"].Set(OrderGetInteger(ORDER_TIME_DONE_MSC));
      response["data"][idx]["type"].Set(OrderGetInteger(ORDER_TYPE));
      response["data"][idx]["type_filling"].Set(OrderGetInteger(ORDER_TYPE_FILLING));
      response["data"][idx]["type_time"].Set(OrderGetInteger(ORDER_TYPE_TIME));
      response["data"][idx]["magic"].Set(OrderGetInteger(ORDER_MAGIC));
      response["data"][idx]["position_id"].Set(OrderGetInteger(ORDER_POSITION_ID));
      response["data"][idx]["volume_initial"].Set(OrderGetDouble(ORDER_VOLUME_INITIAL));
      response["data"][idx]["volume_current"].Set(OrderGetDouble(ORDER_VOLUME_CURRENT));
      response["data"][idx]["price_open"].Set(OrderGetDouble(ORDER_PRICE_OPEN));
      response["data"][idx]["price_current"].Set(OrderGetDouble(ORDER_PRICE_CURRENT));
      response["data"][idx]["price_sl"].Set(OrderGetDouble(ORDER_SL));
      response["data"][idx]["price_tp"].Set(OrderGetDouble(ORDER_TP));
      response["data"][idx]["price_stoplimit"].Set(OrderGetDouble(ORDER_PRICE_STOPLIMIT));
      response["data"][idx]["symbol"].Set(OrderGetString(ORDER_SYMBOL));
      response["data"][idx]["comment"].Set(OrderGetString(ORDER_COMMENT));
      response["data"][idx]["external_id"].Set(OrderGetString(ORDER_EXTERNAL_ID));
      idx++;
   }
}

void HandlePositionsTotal(CJAVal &request, CJAVal &response)
{
   SetSuccess(response);
   response["data"].Set(PositionsTotal());
}

void HandlePositionsGet(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();
   long ticket = request["params"]["ticket"].ToInt();

   SetSuccess(response);
   int total = PositionsTotal();
   int idx = 0;

   for(int i = 0; i < total; i++)
   {
      ulong posTicket = PositionGetTicket(i);
      if(posTicket == 0) continue;

      if(symbol != "" && PositionGetString(POSITION_SYMBOL) != symbol) continue;
      if(ticket > 0 && (long)posTicket != ticket) continue;

      response["data"][idx]["ticket"].Set(posTicket);
      response["data"][idx]["time"].Set(PositionGetInteger(POSITION_TIME));
      response["data"][idx]["time_msc"].Set(PositionGetInteger(POSITION_TIME_MSC));
      response["data"][idx]["time_update"].Set(PositionGetInteger(POSITION_TIME_UPDATE));
      response["data"][idx]["time_update_msc"].Set(PositionGetInteger(POSITION_TIME_UPDATE_MSC));
      response["data"][idx]["type"].Set(PositionGetInteger(POSITION_TYPE));
      response["data"][idx]["magic"].Set(PositionGetInteger(POSITION_MAGIC));
      response["data"][idx]["identifier"].Set(PositionGetInteger(POSITION_IDENTIFIER));
      response["data"][idx]["volume"].Set(PositionGetDouble(POSITION_VOLUME));
      response["data"][idx]["price_open"].Set(PositionGetDouble(POSITION_PRICE_OPEN));
      response["data"][idx]["price_current"].Set(PositionGetDouble(POSITION_PRICE_CURRENT));
      response["data"][idx]["price_sl"].Set(PositionGetDouble(POSITION_SL));
      response["data"][idx]["price_tp"].Set(PositionGetDouble(POSITION_TP));
      response["data"][idx]["swap"].Set(PositionGetDouble(POSITION_SWAP));
      response["data"][idx]["profit"].Set(PositionGetDouble(POSITION_PROFIT));
      response["data"][idx]["symbol"].Set(PositionGetString(POSITION_SYMBOL));
      response["data"][idx]["comment"].Set(PositionGetString(POSITION_COMMENT));
      response["data"][idx]["external_id"].Set(PositionGetString(POSITION_EXTERNAL_ID));
      idx++;
   }
}

void HandleHistoryOrdersTotal(CJAVal &request, CJAVal &response)
{
   datetime dateFrom = (datetime)request["params"]["date_from"].ToInt();
   datetime dateTo = (datetime)request["params"]["date_to"].ToInt();

   if(!HistorySelect(dateFrom, dateTo))
   {
      SetError(response, GetLastError(), "HistorySelect failed");
      return;
   }

   SetSuccess(response);
   response["data"].Set(HistoryOrdersTotal());
}

void HandleHistoryOrdersGet(CJAVal &request, CJAVal &response)
{
   datetime dateFrom = (datetime)request["params"]["date_from"].ToInt();
   datetime dateTo = (datetime)request["params"]["date_to"].ToInt();
   long ticket = request["params"]["ticket"].ToInt();
   long position = request["params"]["position"].ToInt();

   if(ticket > 0)
   {
      if(!HistoryOrderSelect(ticket))
      {
         SetError(response, GetLastError(), "Order not found");
         return;
      }
      SetSuccess(response);
      int idx = 0;
      FillHistoryOrder(response, idx, ticket);
      return;
   }

   if(!HistorySelect(dateFrom, dateTo))
   {
      SetError(response, GetLastError(), "HistorySelect failed");
      return;
   }

   SetSuccess(response);
   int total = HistoryOrdersTotal();
   int idx = 0;

   for(int i = 0; i < total; i++)
   {
      ulong orderTicket = HistoryOrderGetTicket(i);
      if(orderTicket == 0) continue;
      if(position > 0 && HistoryOrderGetInteger(orderTicket, ORDER_POSITION_ID) != position) continue;

      FillHistoryOrder(response, idx, (long)orderTicket);
      idx++;
   }
}

void FillHistoryOrder(CJAVal &response, int idx, long ticket)
{
   response["data"][idx]["ticket"].Set(ticket);
   response["data"][idx]["time_setup"].Set(HistoryOrderGetInteger(ticket, ORDER_TIME_SETUP));
   response["data"][idx]["time_setup_msc"].Set(HistoryOrderGetInteger(ticket, ORDER_TIME_SETUP_MSC));
   response["data"][idx]["time_done"].Set(HistoryOrderGetInteger(ticket, ORDER_TIME_DONE));
   response["data"][idx]["time_done_msc"].Set(HistoryOrderGetInteger(ticket, ORDER_TIME_DONE_MSC));
   response["data"][idx]["type"].Set(HistoryOrderGetInteger(ticket, ORDER_TYPE));
   response["data"][idx]["type_filling"].Set(HistoryOrderGetInteger(ticket, ORDER_TYPE_FILLING));
   response["data"][idx]["type_time"].Set(HistoryOrderGetInteger(ticket, ORDER_TYPE_TIME));
   response["data"][idx]["magic"].Set(HistoryOrderGetInteger(ticket, ORDER_MAGIC));
   response["data"][idx]["position_id"].Set(HistoryOrderGetInteger(ticket, ORDER_POSITION_ID));
   response["data"][idx]["volume_initial"].Set(HistoryOrderGetDouble(ticket, ORDER_VOLUME_INITIAL));
   response["data"][idx]["volume_current"].Set(HistoryOrderGetDouble(ticket, ORDER_VOLUME_CURRENT));
   response["data"][idx]["price_open"].Set(HistoryOrderGetDouble(ticket, ORDER_PRICE_OPEN));
   response["data"][idx]["price_current"].Set(HistoryOrderGetDouble(ticket, ORDER_PRICE_CURRENT));
   response["data"][idx]["price_sl"].Set(HistoryOrderGetDouble(ticket, ORDER_SL));
   response["data"][idx]["price_tp"].Set(HistoryOrderGetDouble(ticket, ORDER_TP));
   response["data"][idx]["symbol"].Set(HistoryOrderGetString(ticket, ORDER_SYMBOL));
   response["data"][idx]["comment"].Set(HistoryOrderGetString(ticket, ORDER_COMMENT));
   response["data"][idx]["external_id"].Set(HistoryOrderGetString(ticket, ORDER_EXTERNAL_ID));
}

void HandleHistoryDealsTotal(CJAVal &request, CJAVal &response)
{
   datetime dateFrom = (datetime)request["params"]["date_from"].ToInt();
   datetime dateTo = (datetime)request["params"]["date_to"].ToInt();

   if(!HistorySelect(dateFrom, dateTo))
   {
      SetError(response, GetLastError(), "HistorySelect failed");
      return;
   }

   SetSuccess(response);
   response["data"].Set(HistoryDealsTotal());
}

void HandleHistoryDealsGet(CJAVal &request, CJAVal &response)
{
   datetime dateFrom = (datetime)request["params"]["date_from"].ToInt();
   datetime dateTo = (datetime)request["params"]["date_to"].ToInt();
   long ticket = request["params"]["ticket"].ToInt();
   long position = request["params"]["position"].ToInt();

   if(ticket > 0)
   {
      if(!HistoryDealSelect(ticket))
      {
         SetError(response, GetLastError(), "Deal not found");
         return;
      }
      SetSuccess(response);
      int idx = 0;
      FillHistoryDeal(response, idx, ticket);
      return;
   }

   if(!HistorySelect(dateFrom, dateTo))
   {
      SetError(response, GetLastError(), "HistorySelect failed");
      return;
   }

   SetSuccess(response);
   int total = HistoryDealsTotal();
   int idx = 0;

   for(int i = 0; i < total; i++)
   {
      ulong dealTicket = HistoryDealGetTicket(i);
      if(dealTicket == 0) continue;
      if(position > 0 && HistoryDealGetInteger(dealTicket, DEAL_POSITION_ID) != position) continue;

      FillHistoryDeal(response, idx, (long)dealTicket);
      idx++;
   }
}

void FillHistoryDeal(CJAVal &response, int idx, long ticket)
{
   response["data"][idx]["ticket"].Set(ticket);
   response["data"][idx]["order"].Set(HistoryDealGetInteger(ticket, DEAL_ORDER));
   response["data"][idx]["time"].Set(HistoryDealGetInteger(ticket, DEAL_TIME));
   response["data"][idx]["time_msc"].Set(HistoryDealGetInteger(ticket, DEAL_TIME_MSC));
   response["data"][idx]["type"].Set(HistoryDealGetInteger(ticket, DEAL_TYPE));
   response["data"][idx]["entry"].Set(HistoryDealGetInteger(ticket, DEAL_ENTRY));
   response["data"][idx]["magic"].Set(HistoryDealGetInteger(ticket, DEAL_MAGIC));
   response["data"][idx]["position_id"].Set(HistoryDealGetInteger(ticket, DEAL_POSITION_ID));
   response["data"][idx]["volume"].Set(HistoryDealGetDouble(ticket, DEAL_VOLUME));
   response["data"][idx]["price"].Set(HistoryDealGetDouble(ticket, DEAL_PRICE));
   response["data"][idx]["commission"].Set(HistoryDealGetDouble(ticket, DEAL_COMMISSION));
   response["data"][idx]["swap"].Set(HistoryDealGetDouble(ticket, DEAL_SWAP));
   response["data"][idx]["profit"].Set(HistoryDealGetDouble(ticket, DEAL_PROFIT));
   response["data"][idx]["fee"].Set(HistoryDealGetDouble(ticket, DEAL_FEE));
   response["data"][idx]["symbol"].Set(HistoryDealGetString(ticket, DEAL_SYMBOL));
   response["data"][idx]["comment"].Set(HistoryDealGetString(ticket, DEAL_COMMENT));
   response["data"][idx]["external_id"].Set(HistoryDealGetString(ticket, DEAL_EXTERNAL_ID));
}

void HandleSubscribeTicks(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();

   for(int i = 0; i < ArraySize(subscribedSymbols); i++)
   {
      if(subscribedSymbols[i] == symbol)
      {
         SetSuccess(response);
         return;
      }
   }

   int size = ArraySize(subscribedSymbols);
   ArrayResize(subscribedSymbols, size + 1);
   subscribedSymbols[size] = symbol;

   SetSuccess(response);
}

void HandleUnsubscribeTicks(CJAVal &request, CJAVal &response)
{
   string symbol = request["params"]["symbol"].ToStr();

   int size = ArraySize(subscribedSymbols);
   for(int i = 0; i < size; i++)
   {
      if(subscribedSymbols[i] == symbol)
      {
         for(int j = i; j < size - 1; j++)
            subscribedSymbols[j] = subscribedSymbols[j + 1];
         ArrayResize(subscribedSymbols, size - 1);
         break;
      }
   }

   SetSuccess(response);
}
