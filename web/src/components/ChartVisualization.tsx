import React from "react";
import { ChartData } from "../types/chart";

interface ChartVisualizationProps {
  type: string;
  title: string;
  dataset: ChartData["dataset"];
  caption?: string;
}

interface ChartVisualizationState {
  error?: string;
}

export const ChartVisualization: React.FC<ChartVisualizationProps> = ({
  type: _type,
  title,
  dataset: _dataset,
  caption,
}) => {
  const [state] = React.useState<ChartVisualizationState>({});

  if (state.error) {
    return (
      <div
        style={{
          padding: "12px",
          backgroundColor: "#fff5f5",
          border: "1px solid #feb2b2",
          borderRadius: "4px",
          color: "#c53030",
          fontSize: "13px",
        }}
      >
        Failed to render chart: {state.error}
      </div>
    );
  }

  return (
    <div
      style={{
        padding: "12px",
        border: "1px solid #2a2e36",
        borderRadius: "4px",
        backgroundColor: "#14161a",
      }}
    >
      <div style={{ color: "#e8e6e3", fontSize: "14px", marginBottom: "8px" }}>
        {title}
      </div>
      <div>Chart component to be implemented</div>
      {caption && (
        <div
          style={{
            marginTop: "8px",
            color: "#8a8680",
            fontSize: "12px",
            fontStyle: "italic",
          }}
        >
          {caption}
        </div>
      )}
    </div>
  );
};
