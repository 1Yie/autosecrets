import { lazy, Suspense, useEffect, useState } from "react";
import { Outlet } from "react-router-dom";
import { ShieldCheck } from "lucide-react";

const Beams = lazy(() => import("../components/Beams"));

function canUseWebGL(): boolean {
  const canvas = document.createElement("canvas");
  return Boolean(canvas.getContext("webgl2") ?? canvas.getContext("webgl"));
}

function useBeamsEnabled(): boolean {
  const [enabled, setEnabled] = useState(false);

  useEffect(() => {
    const media = window.matchMedia("(prefers-reduced-motion: reduce)");
    const update = () => {
      setEnabled(!media.matches && canUseWebGL());
    };
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, []);

  return enabled;
}

/** AuthLayout: the left-right authentication shell for the /auth/* routes
 * (login, bootstrap, and resumed MFA enrollment). The left panel is a Beams
 * background; the right panel carries the form via <Outlet />. */
export function AuthLayout() {
  const beamsEnabled = useBeamsEnabled();

  return (
    <div className="grid min-h-dvh md:grid-cols-[1fr_1.1fr]">
      <div className="relative hidden overflow-hidden bg-black text-white md:block">
        {beamsEnabled && (
          <div className="absolute inset-0" aria-hidden="true">
            <Suspense fallback={null}>
              <Beams
                beamWidth={2}
                beamHeight={15}
                beamNumber={12}
                lightColor="#ffffff"
                speed={2}
                noiseIntensity={1.75}
                scale={0.2}
                rotation={30}
              />
            </Suspense>
          </div>
        )}
        <div className="relative z-10 flex items-center gap-2 p-10">
          <ShieldCheck className="size-5" />
          <span className="font-semibold tracking-tight">AutoSecrets</span>
        </div>
      </div>
      <div className="flex items-center justify-center p-6">
        <div className="w-full max-w-md space-y-6">
          <Outlet />
        </div>
      </div>
    </div>
  );
}

export default AuthLayout;
