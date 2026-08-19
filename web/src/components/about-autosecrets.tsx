import { useQuery } from "@tanstack/react-query";
import { ExternalLink, GitBranch, Heart, Package, Users } from "lucide-react";
import { apiGet } from "../lib/api";
import { API_PATHS } from "../lib/constants/api-paths";
import { APP_VERSION } from "../lib/env";
import { Avatar, AvatarFallback, AvatarImage } from "./ui/avatar";
import { Badge } from "./ui/badge";
import { Button } from "./ui/button";
import { Frame, FramePanel } from "./ui/frame";
import { Skeleton } from "./ui/skeleton";

interface Health {
  status: string;
  service: string;
}

interface Person {
  name: string;
  role: string;
  url?: string;
  github?: string;
}

function githubAvatarUrl(username: string): string {
  return `https://github.com/${username}.png?size=80`;
}

function PersonList({ people }: { people: Person[] }) {
  return (
    <ul className="w-fit max-w-full space-y-2">
      {people.map((person) => (
        <li
          key={person.name}
          className="flex w-fit max-w-full items-center gap-3 rounded-lg border p-3"
        >
          <Avatar className="size-9">
            {person.github ? (
              <AvatarImage
                src={githubAvatarUrl(person.github)}
                alt={`${person.name} 的头像`}
                referrerPolicy="no-referrer"
              />
            ) : null}
            <AvatarFallback>
              {person.name.slice(0, 1).toUpperCase()}
            </AvatarFallback>
          </Avatar>
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">{person.name}</div>
            <div className="truncate text-xs text-muted-foreground">
              {person.role}
            </div>
          </div>
          {person.url && (
            <Button
              size="icon-sm"
              variant="ghost"
              aria-label={`${person.name} 主页`}
              className="ml-auto shrink-0 text-muted-foreground"
              onClick={() =>
                window.open(person.url, "_blank", "noopener,noreferrer")
              }
            >
              <ExternalLink className="size-3.5" />
            </Button>
          )}
        </li>
      ))}
    </ul>
  );
}

const REPO_URL = "https://github.com/1Yie/autosecrets";

const CONTRIBUTORS: Person[] = [
  {
    name: "1Yie",
    role: "作者",
    url: "https://github.com/1Yie",
    github: "1Yie",
  },
];

const THANKS: Person[] = [
  {
    name: "kmou424",
    role: "原始作者",
    url: "https://github.com/kmou424",
    github: "kmou424",
  },
];

export function AboutAutosecrets() {
  const health = useQuery({
    queryKey: ["health"],
    queryFn: () => apiGet<Health>(API_PATHS.health),
    retry: false,
  });

  return (
    <div className="space-y-5">
      <Frame>
        <FramePanel>
          <div className="flex items-center justify-between gap-4">
            <div className="flex min-w-0 items-center gap-3">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted">
                <Package className="size-4 text-muted-foreground" />
              </span>
              <div className="min-w-0">
                <div className="truncate text-sm font-medium">版本</div>
                <div className="truncate text-xs text-muted-foreground">
                  {health.data ? `${health.data.service} 服务` : "核心服务"}
                </div>
              </div>
            </div>
            <div className="flex shrink-0 items-center gap-1.5">
              <Badge variant="secondary" className="font-mono">
                {APP_VERSION}
              </Badge>
              {health.isPending ? (
                <Skeleton className="h-5 w-12 shrink-0" />
              ) : health.data ? (
                <Badge variant="success">正常</Badge>
              ) : (
                <span className="shrink-0 text-xs text-muted-foreground">
                  不可用
                </span>
              )}
            </div>
          </div>
        </FramePanel>
      </Frame>

      <div className="w-fit max-w-full space-y-3">
        <div className="flex items-center gap-2">
          <Users className="size-4 text-muted-foreground" />
          <h3 className="text-sm font-medium">GitHub 贡献者</h3>
        </div>
        <PersonList people={CONTRIBUTORS} />
      </div>

      <div className="w-fit max-w-full space-y-3">
        <div className="flex items-center gap-2">
          <Heart className="size-4 text-muted-foreground" />
          <h3 className="text-sm font-medium">感谢</h3>
        </div>
        <PersonList people={THANKS} />
      </div>

      <Button
        variant="outline"
        className="h-auto w-fit max-w-full justify-start gap-2 p-3 text-sm font-medium"
        onClick={() => window.open(REPO_URL, "_blank", "noopener,noreferrer")}
      >
        <span className="flex min-w-0 items-center gap-2">
          <GitBranch className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate">{REPO_URL.replace("https://", "")}</span>
        </span>
        <ExternalLink className="size-3.5 shrink-0 text-muted-foreground" />
      </Button>
    </div>
  );
}
