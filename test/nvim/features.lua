-- Headless Neovim integration check for martian-lsp features.
-- Invoked by run.sh (which sets MRLSP and MRO_EXAMPLES). Exits non-zero on failure.
local EX = assert(vim.env.MRO_EXAMPLES, "MRO_EXAMPLES not set")
local CMD = assert(vim.env.MRLSP, "MRLSP not set")

vim.filetype.add { extension = { mro = "mro" } }
vim.lsp.config("martian", { cmd = { CMD }, filetypes = { "mro" }, root_markers = { ".git" } })
vim.lsp.enable "martian"

local function out(s) io.stdout:write(s .. "\n") end
local fails = 0
local function check(name, cond, detail)
  if cond then out("PASS  " .. name)
  else fails = fails + 1; out("FAIL  " .. name .. "  -- " .. tostring(detail)) end
end

local function open(path)
  vim.cmd.edit { args = { path }, bang = true } -- discard any unsaved test mangling
  local buf = vim.api.nvim_get_current_buf()
  vim.bo[buf].filetype = "mro" -- `nvim -l` has filetype detection off
  vim.wait(10000, function() return #vim.lsp.get_clients({ bufnr = buf }) > 0 end, 50)
  return buf
end

-- locate a token: 0-indexed line, plus 0-indexed char `off` columns into it
local function locate(buf, lineContains, token, off)
  for i, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
    if line:find(lineContains, 1, true) then
      local c = line:find(token, 1, true)
      if c then return i - 1, c - 1 + (off or 1) end
    end
  end
end

local function request(buf, method, line, char)
  local res = vim.lsp.buf_request_sync(buf, method,
    { textDocument = { uri = vim.uri_from_bufnr(buf) }, position = { line = line, character = char } }, 4000)
  if not res then return nil end
  for _, r in pairs(res) do if r.result then return r.result end end
end

-- diagnostics
do
  local buf = open(EX .. "/diagnostics.mro")
  check("client attached", #vim.lsp.get_clients({ bufnr = buf }) > 0, "no client")
  vim.wait(4000, function() return #vim.diagnostic.get(buf) > 0 end, 50)
  check("diagnostics reported", #vim.diagnostic.get(buf) > 0, "#=" .. #vim.diagnostic.get(buf))
end

-- hover / definition / symbols on hello.mro
do
  local buf = open(EX .. "/hello.mro")
  vim.wait(1500)

  local l, c = locate(buf, "call HELLO(", "HELLO")
  local hov = request(buf, "textDocument/hover", l, c)
  local hv = hov and hov.contents and (hov.contents.value or "") or ""
  check("hover on call -> stage sig", hv:find("stage HELLO", 1, true) ~= nil, hv)

  local def = request(buf, "textDocument/definition", l, c)
  if def and def[1] then def = def[1] end
  check("definition of call -> stage decl", def and def.range and def.range.start.line == 10,
    def and vim.inspect(def.range) or "nil")

  -- output part of HELLO.greeting (offset past "HELLO.")
  local l2, c2 = locate(buf, "greeting = HELLO.greeting", "HELLO.greeting", #"HELLO." + 1)
  local def2 = request(buf, "textDocument/definition", l2, c2)
  if def2 and def2[1] then def2 = def2[1] end
  check("definition of .greeting -> out decl", def2 and def2.range and def2.range.start.line == 12,
    def2 and vim.inspect(def2.range) or "nil")

  local syms = request(buf, "textDocument/documentSymbol", 0, 0)
  local names = {}
  if syms then for _, s in ipairs(syms) do names[s.name] = true end end
  check("symbols include HELLO/SHOUT/HELLO_WORLD",
    names.HELLO and names.SHOUT and names.HELLO_WORLD, vim.inspect(vim.tbl_keys(names)))
end

-- references / rename on hello.mro
do
  local buf = open(EX .. "/hello.mro")
  vim.wait(1500)
  local sl, sc = locate(buf, "stage HELLO(", "HELLO")

  local refs = vim.lsp.buf_request_sync(buf, "textDocument/references", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
    position = { line = sl, character = sc },
    context = { includeDeclaration = true },
  }, 4000)
  local n = 0
  if refs then for _, r in pairs(refs) do if r.result then n = #r.result end end end
  -- stage decl + `call HELLO` + `HELLO.greeting` call-part = 3
  check("references of HELLO = 3", n == 3, "n=" .. n)

  local ren = vim.lsp.buf_request_sync(buf, "textDocument/rename", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
    position = { line = sl, character = sc },
    newName = "HI",
  }, 4000)
  local edits = 0
  if ren then
    for _, r in pairs(ren) do
      if r.result and r.result.changes then
        for _, list in pairs(r.result.changes) do edits = edits + #list end
      end
    end
  end
  check("rename HELLO produces 3 edits", edits == 3, "edits=" .. edits)
end

-- semantic tokens + formatting
do
  local buf = open(EX .. "/hello.mro")
  vim.wait(1000)

  local st = request(buf, "textDocument/semanticTokens/full", 0, 0)
  local n = 0
  if st and st.data then n = #st.data end
  check("semantic tokens produced (data multiple of 5)", n > 0 and n % 5 == 0, "n=" .. n)

  -- Mangle a line, then format: expect edits back.
  for i, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
    if line:find("in  string name", 1, true) then
      vim.api.nvim_buf_set_lines(buf, i - 1, i, false, { "in string name," })
      break
    end
  end
  vim.wait(500)
  local fmt = vim.lsp.buf_request_sync(buf, "textDocument/formatting", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
    options = { tabSize = 4, insertSpaces = true },
  }, 4000)
  local edits = 0
  if fmt then for _, r in pairs(fmt) do if r.result then edits = #r.result end end end
  check("formatting returns edits for mangled doc", edits > 0, "edits=" .. edits)
end

-- completion
do
  local buf = open(EX .. "/hello.mro")
  vim.wait(1000)

  local function colAfter(pat, sub)
    for i, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
      if line:find(pat, 1, true) then
        local c = line:find(sub, 1, true)
        if c then return i - 1, c - 1 + #sub end
      end
    end
  end
  local function compLabels(line, char)
    local r = vim.lsp.buf_request_sync(buf, "textDocument/completion", {
      textDocument = { uri = vim.uri_from_bufnr(buf) },
      position = { line = line, character = char },
      context = { triggerKind = 1 },
    }, 4000)
    local labels = {}
    if r then
      for _, x in pairs(r) do
        local res = x.result
        local items = res and (res.items or res) or {}
        for _, it in ipairs(items) do labels[it.label] = true end
      end
    end
    return labels
  end

  local ol, oc = colAfter("greeting = HELLO.greeting", "HELLO.")
  check("completion after HELLO. includes 'greeting'", compLabels(ol, oc).greeting == true, "")

  local tl, tc = colAfter("in  string name", "in  ")
  check("completion in type position includes 'string'", compLabels(tl, tc).string == true, "")
end

-- Tier 1 features: documentHighlight, prepareRename, signatureHelp, typeDefinition
do
  local buf = open(EX .. "/hello.mro")
  vim.wait(1000)
  local function colOf(pat, sub)
    for i, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
      if line:find(pat, 1, true) then
        local c = line:find(sub, 1, true)
        if c then return i - 1, c - 1 end
      end
    end
  end

  local hl, hc = colOf("stage HELLO(", "HELLO")
  local highlights = request(buf, "textDocument/documentHighlight", hl, hc + 1)
  check("documentHighlight returns occurrences", highlights and #highlights >= 2, "")

  local pr = request(buf, "textDocument/prepareRename", hl, hc + 1)
  check("prepareRename returns a range", pr ~= nil and pr.start ~= nil, "")

  local sl, sc = colOf("call HELLO(", "(")
  local sig = request(buf, "textDocument/signatureHelp", sl, sc + 1)
  local label = sig and sig.signatures and sig.signatures[1] and sig.signatures[1].label or ""
  check("signatureHelp shows HELLO signature", label:find("HELLO(", 1, true) ~= nil, label)

  local dl, dc = colOf("txt    loud", "loud")
  local td = request(buf, "textDocument/typeDefinition", dl, dc + 1)
  if td and td[1] then td = td[1] end
  check("typeDefinition resolves txt-typed param", td ~= nil and td.uri ~= nil, "")
end

-- document links on @include
do
  local buf = open(EX .. "/include/aligner.mro")
  vim.wait(800)
  local dl = vim.lsp.buf_request_sync(buf, "textDocument/documentLink", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
  }, 4000)
  local n = 0
  if dl then for _, x in pairs(dl) do if x.result then n = #x.result end end end
  check("documentLink on @include", n >= 1, "n=" .. n)
end

-- call hierarchy
do
  local buf = open(EX .. "/hello.mro")
  vim.wait(1000)
  local function colOf(pat, sub)
    for i, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
      if line:find(pat, 1, true) then
        local c = line:find(sub, 1, true)
        if c then return i - 1, c - 1 end
      end
    end
  end
  local function names(result, field)
    local m = {}
    if result then
      for _, x in pairs(result) do
        if x.result then
          for _, c in ipairs(x.result) do m[c[field].name] = true end
        end
      end
    end
    return m
  end

  local hl, hc = colOf("stage HELLO(", "HELLO")
  local prep = request(buf, "textDocument/prepareCallHierarchy", hl, hc + 1)
  local item = prep and prep[1]
  check("prepareCallHierarchy returns HELLO", item ~= nil and item.name == "HELLO", item and item.name or "nil")
  if item then
    local inc = vim.lsp.buf_request_sync(buf, "callHierarchy/incomingCalls", { item = item }, 4000)
    check("incomingCalls(HELLO) includes HELLO_WORLD", names(inc, "from").HELLO_WORLD == true, "")
  end

  local pl, pc = colOf("pipeline HELLO_WORLD(", "HELLO_WORLD")
  local prepP = request(buf, "textDocument/prepareCallHierarchy", pl, pc + 1)
  local pItem = prepP and prepP[1]
  if pItem then
    local out = vim.lsp.buf_request_sync(buf, "callHierarchy/outgoingCalls", { item = pItem }, 4000)
    local callees = names(out, "to")
    check("outgoingCalls(HELLO_WORLD) includes HELLO and SHOUT", callees.HELLO and callees.SHOUT, "")
  end
end

-- inlay hints + code actions
do
  local buf = open(EX .. "/hello.mro")
  vim.wait(1000)

  local ih = vim.lsp.buf_request_sync(buf, "textDocument/inlayHint", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
    range = { start = { line = 0, character = 0 }, ["end"] = { line = 200, character = 0 } },
  }, 4000)
  local nh, hasType = 0, false
  if ih then
    for _, x in pairs(ih) do
      if x.result then
        for _, h in ipairs(x.result) do
          nh = nh + 1
          if type(h.label) == "string" and h.label:find(":", 1, true) then hasType = true end
        end
      end
    end
  end
  check("inlay hints produced with type labels", nh >= 1 and hasType, "n=" .. nh)

  -- Mangle the buffer to call an undefined stage, then request a code action.
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, {
    "pipeline P(", ")", "{", "    call NOPE(", "        ", "    )", "    return ()", "}",
  })
  vim.wait(800)
  local ca = vim.lsp.buf_request_sync(buf, "textDocument/codeAction", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
    range = { start = { line = 4, character = 8 }, ["end"] = { line = 4, character = 8 } },
    context = { diagnostics = {} },
  }, 4000)
  local titles = {}
  if ca then
    for _, x in pairs(ca) do
      if x.result then
        for _, a in ipairs(x.result) do titles[a.title] = true end
      end
    end
  end
  check("code action offers Create stage NOPE", titles["Create stage NOPE"] == true, "")

  -- add-missing-arguments on a known stage call with no args
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, {
    "stage S(", "    in  int n,", "    out int y,", "    src comp \"s\",", ")",
    "pipeline P(", ")", "{", "    call S(", "        ", "    )", "    return ()", "}",
  })
  vim.wait(800)
  local ca2 = vim.lsp.buf_request_sync(buf, "textDocument/codeAction", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
    range = { start = { line = 9, character = 8 }, ["end"] = { line = 9, character = 8 } },
    context = { diagnostics = {} },
  }, 4000)
  local t2 = {}
  if ca2 then for _, x in pairs(ca2) do if x.result then for _, a in ipairs(x.result) do t2[a.title] = true end end end end
  check("code action offers Add missing arguments", t2["Add missing arguments"] == true, "")
end

-- src-file go-to-definition + snippet completion
do
  local buf = open(EX .. "/hello.mro")
  vim.wait(1000)

  local function findCol(sub)
    for i, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
      local c = line:find(sub, 1, true)
      if c then return i - 1, c - 1 end
    end
  end
  local sl, sc = findCol("stages/hello")
  local def = request(buf, "textDocument/definition", sl, sc + 2)
  if def and def[1] then def = def[1] end
  check("src definition resolves to stages/hello", def ~= nil and def.uri:find("stages/hello", 1, true) ~= nil, "")

  vim.api.nvim_buf_set_lines(buf, 0, -1, false, { "stage" })
  vim.wait(400)
  local comp = vim.lsp.buf_request_sync(buf, "textDocument/completion", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
    position = { line = 0, character = 5 },
    context = { triggerKind = 1 },
  }, 4000)
  local isSnippet = false
  if comp then
    for _, x in pairs(comp) do
      local items = x.result and (x.result.items or x.result) or {}
      for _, it in ipairs(items) do
        if it.label == "stage" and it.insertTextFormat == 2 then isSnippet = true end
      end
    end
  end
  check("snippet completion for stage", isSnippet, "")
end

-- workspace symbols + cross-file references
do
  local buf = open(EX .. "/include/aligner.mro")
  vim.wait(1500)

  local sym = vim.lsp.buf_request_sync(buf, "workspace/symbol", { query = "ALIGN" }, 4000)
  local found = {}
  if sym then
    for _, x in pairs(sym) do
      if x.result then for _, s in ipairs(x.result) do found[s.name] = true end end
    end
  end
  check("workspace/symbol finds ALIGN", found.ALIGN == true, "")

  local cl, cc = locate(buf, "call ALIGN(", "ALIGN")
  local rr = vim.lsp.buf_request_sync(buf, "textDocument/references", {
    textDocument = { uri = vim.uri_from_bufnr(buf) },
    position = { line = cl, character = cc },
    context = { includeDeclaration = true },
  }, 4000)
  local inDna, n = false, 0
  if rr then
    for _, x in pairs(rr) do
      if x.result then
        for _, loc in ipairs(x.result) do
          n = n + 1
          if loc.uri:find("dna.mro", 1, true) then inDna = true end
        end
      end
    end
  end
  check("cross-file references reach dna.mro", inDna, "n=" .. n)
end

-- cross-file definition
do
  local buf = open(EX .. "/include/aligner.mro")
  vim.wait(1500)
  local l, c = locate(buf, "aligned = ALIGN.aligned", "ALIGN.aligned")
  local def = request(buf, "textDocument/definition", l, c)
  if def and def[1] then def = def[1] end
  local uri = def and def.uri or ""
  check("cross-file definition lands in dna.mro", uri:find("dna.mro", 1, true) ~= nil, uri)
end

out(fails == 0 and "\nALL FEATURE CHECKS PASSED" or ("\n" .. fails .. " CHECK(S) FAILED"))
vim.cmd("cquit " .. (fails == 0 and 0 or 1))
