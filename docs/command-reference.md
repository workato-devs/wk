# Command Reference

Every command supports `--help` for detailed usage and flags.

```
wk
  auth
    login           Create or update an auth profile
    list            List all auth profiles
    switch          Switch active profile
    status          Show active profile and test connectivity
    delete          Delete an auth profile and its stored credential
    token           Print the active profile's API token
  init              Initialize a new wk project
  link              Link the current project to an auth profile
  clone             Clone a remote folder into a new local project
  recipes (recipe)
    list            List recipes
    get             Get recipe details
    start           Start one or more recipes
    stop            Stop one or more recipes
    export          Export a recipe as JSON
    import          Import a recipe from JSON file
    update          Update an existing recipe from a JSON file
    delete          Delete a recipe (server and/or local)
    move            Move a recipe to a different folder
    copy            Copy a recipe to a folder
    pull            Pull a single recipe by ID (bypasses package export)
    update-connection  Update a recipe's connection
    validate        Validate recipe files (delegates to recipe-lint plugin)
    jobs            List recipe jobs
      get           Get details for a single job, including step traces
      retry         Retry (repeat) one or more jobs using the latest recipe version
    versions        Manage recipe version history
      comment       Set or update the comment on a recipe version
  connections (conn)
    list            List connections
    get             Get connection details
    create          Create a connection
    update          Update a connection
    delete          Delete a connection
    disconnect      Disconnect a connection
  connectors (connector)
    list            List custom SDK connectors (--search to filter)
  folders (folder)
    list            List folders (--projects to list projects)
    create          Create a folder
    update          Rename a folder or project
    delete          Delete a folder or project
  tags (tag)
    list            List tags
    create          Create a tag
    update          Update a tag
    delete          Delete a tag
    apply           Apply a tag to recipes or connections
    remove          Remove a tag from recipes or connections
  api
    collections (collection)
      list          List API collections
      create        Create an API collection
      delete        Delete an API collection
    endpoints (endpoint)
      list          List API endpoints
      create        Create an API endpoint from a JSON file
      create-batch  Create API endpoints in batch from a directory or CSV
      enable        Enable an API endpoint
      disable       Disable an API endpoint
    clients (client)
      list          List API Platform clients
      get           Get an API Platform client
      create        Create an API Platform client
      delete        Delete an API Platform client
      keys (key)
        create      Create an API key for a client
        refresh     Rotate an API key (generates a new auth token)
  agentic
    skills (skill)
      list          List agentic skills
      get           Get an agentic skill by ID
      create        Create an agentic skill from a recipe
  mcp
    test            Test MCP server connectivity
    tools           List tools exposed by an MCP server
    servers (server)
      list          List MCP servers
      get           Get MCP server details
      create        Create an MCP server
      create-batch  Create MCP servers in batch from a manifest or CSV
      update        Update an MCP server
      delete        Delete an MCP server
      token-renew   Renew the authentication token for an MCP server
      tools
        list        List tools assigned to an MCP server
        add         Assign recipes as tools to an MCP server
        update      Update an assigned tool (e.g. its description)
        remove      Remove a tool from an MCP server
      policies
        get         Get an MCP server's policy
        set         Update an MCP server's policy
      user-groups (user-group)
        add         Grant IdP user groups access to an MCP server
        remove      Revoke IdP user groups from an MCP server
    user-groups (user-group)
      list          List IdP user groups (resolve IDs for server user-group grants)
  workspace
    info            Show current workspace info
    users           List workspace members
    audit-log       View workspace audit log
    properties
      list          List environment properties
      set           Set environment properties (upsert)
  sync
    add             Add one or more [[sync]] entries to wk.toml
    list            List configured sync entries
    discover        List server-side folders not yet in sync config
    refresh         Reconcile cached folder_id values against the workspace
    remove          Remove a [[sync]] entry from wk.toml
  pull              Pull remote assets to local project
  push              Push local changes to remote workspace
  status            Show sync status of the current project
  diff              Show differences between local and remote
  plugins (plugin)
    install         Install a plugin by name (from $PATH) or local path
    list            List installed plugins
    remove          Remove an installed plugin
  version           Print the wk CLI version
  completion        Generate shell completions (bash, zsh, fish, powershell)
```

The `lint` command is contributed by the recipe-lint plugin when installed
(`wk lint` is equivalent to `wk recipes validate`; `wk lint version` shows the
installed plugin version).

## Global flags

Available on all commands:

| Flag | Description |
|---|---|
| `--json`, `-j` | Output as JSON |
| `--verbose` | Enable debug logging |
| `--quiet`, `-q` | Suppress non-essential output |
| `--profile`, `-p` | Override active workspace profile |
| `--store-type <backend>` | Override credential store backend (`keychain`\|`file`) |
| `--no-color` | Disable color output |
| `--no-input` | Force non-interactive mode |
| `--timeout <secs>` | API timeout in seconds (default 30) |
