import type { ComponentType } from "react";

export interface BeamsProps {
  beamWidth?: number;
  beamHeight?: number;
  beamNumber?: number;
  lightColor?: string;
  speed?: number;
  noiseIntensity?: number;
  scale?: number;
  rotation?: number;
}

declare const Beams: ComponentType<BeamsProps>;
export default Beams;
