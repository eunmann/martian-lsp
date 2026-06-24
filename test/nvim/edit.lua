-- Headless Neovim check: diagnostics refresh on EDIT (didChange/full-sync path).
-- Invoked by run.sh. Exits non-zero on failure.
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

vim.cmd.edit(EX .. "/hello.mro")
local buf = vim.api.nvim_get_current_buf()
vim.bo[buf].filetype = "mro"
vim.wait(10000, function() return #vim.lsp.get_clients({ bufnr = buf }) > 0 end, 50)

local function lineOf(text)
  for i, line in ipairs(vim.api.nvim_buf_get_lines(buf, 0, -1, false)) do
    if line:find(text, 1, true) then return i - 1, line end
  end
end

vim.wait(3000)
check("initially 0 diagnostics", #vim.diagnostic.get(buf) == 0, "#=" .. #vim.diagnostic.get(buf))

-- edit introduces an error
local idx, line = lineOf("greeting = HELLO.greeting")
vim.api.nvim_buf_set_lines(buf, idx, idx + 1, false, { (line:gsub("HELLO%.greeting", "HELLO.nope")) })
check("edit introduces diagnostic",
  vim.wait(5000, function() return #vim.diagnostic.get(buf) > 0 end, 50), "#=" .. #vim.diagnostic.get(buf))

-- edit fixes it
vim.api.nvim_buf_set_lines(buf, idx, idx + 1, false, { line })
check("edit clears diagnostic",
  vim.wait(5000, function() return #vim.diagnostic.get(buf) == 0 end, 50), "#=" .. #vim.diagnostic.get(buf))

out(fails == 0 and "\nALL EDIT CHECKS PASSED" or ("\n" .. fails .. " CHECK(S) FAILED"))
vim.cmd("cquit " .. (fails == 0 and 0 or 1))
