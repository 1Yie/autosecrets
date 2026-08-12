import { useState } from "react";
import { Link } from "react-router-dom";
import { Search } from "lucide-react";
import { useSearch, type SearchResult } from "../hooks/fleet/use-search";
import { Input } from "./ui/input";

const typeLabels: Record<SearchResult["type"], string> = {
  application: "应用",
  environment: "环境",
  node: "节点",
  node_group: "节点组",
};

const typeTargets: Record<SearchResult["type"], (result: SearchResult) => string> = {
  application: (result) => `/apps/${result.id}`,
  environment: () => "/apps",
  node: () => "/nodes",
  node_group: () => "/nodes",
};

/** Global search: Applications, Environments, Managed Nodes, Node Groups. */
export function SearchBox() {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState(false);
  const search = useSearch(query);
  const results = search.data?.results ?? [];

  return (
    <div className="relative">
      <div className="relative">
        <Search className="absolute left-2 top-1/2 size-3.5 -translate-y-1/2 opacity-50" />
        <Input
          className="w-56 pl-7"
          placeholder="搜索…"
          value={query}
          data-testid="global-search"
          onChange={(event) => {
            setQuery(event.target.value);
            setOpen(true);
          }}
          onBlur={() => setTimeout(() => setOpen(false), 150)}
          onFocus={() => setOpen(true)}
        />
      </div>
      {open && query.trim().length >= 2 && (
        <div className="absolute right-0 top-full z-20 mt-1 w-72 rounded-lg border bg-background p-2 shadow-lg" data-testid="search-results">
          {search.isLoading && <p className="px-2 py-1 text-sm opacity-60">搜索中…</p>}
          {!search.isLoading && results.length === 0 && (
            <p className="px-2 py-1 text-sm opacity-60">无结果</p>
          )}
          {results.map((result) => (
            <Link
              key={`${result.type}-${result.id}`}
              to={typeTargets[result.type](result)}
              className="flex items-center justify-between rounded px-2 py-1 text-sm hover:bg-muted"
            >
              <span className="truncate">{result.name}</span>
              <span className="ml-2 shrink-0 text-xs opacity-50">{typeLabels[result.type]}</span>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
