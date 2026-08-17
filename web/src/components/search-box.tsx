import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { Search } from "lucide-react";
import { useSearch, type SearchResult } from "../hooks/fleet/use-search";
import { InputGroup, InputGroupAddon, InputGroupInput } from "./ui/input-group";

const typeLabels: Record<SearchResult["type"], string> = {
  application: "应用",
  environment: "环境",
  node: "节点",
  node_group: "节点组",
};

const typeTargets: Record<SearchResult["type"], (result: SearchResult) => string> = {
  application: (result) => `/dashboard/apps/${result.id}`,
  environment: () => "/dashboard/apps",
  node: () => "/dashboard/nodes",
  node_group: () => "/dashboard/nodes",
};

/** Global search: Applications, Environments, Managed Nodes, Node Groups. */
export function SearchBox() {
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [open, setOpen] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => setDebouncedQuery(query), 300);
    return () => clearTimeout(timer);
  }, [query]);

  const search = useSearch(debouncedQuery);
  const results = search.data?.results ?? [];
  const pending = query.trim() !== debouncedQuery.trim() || search.isLoading;

  return (
    <div className="relative">
      <InputGroup className="w-56">
        <InputGroupAddon className="ps-2.5 pe-1.5">
          <Search className="size-4" aria-hidden="true" />
        </InputGroupAddon>
        <InputGroupInput
          className="**:data-[slot=input]:ps-0"
          type="search"
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
      </InputGroup>
      {open && query.trim().length >= 2 && (
        <div className="absolute right-0 top-full z-20 mt-1 w-72 rounded-lg border bg-background p-2 shadow-lg" data-testid="search-results">
          {pending && <p className="px-2 py-1 text-sm opacity-60">搜索中…</p>}
          {!pending && results.length === 0 && (
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
