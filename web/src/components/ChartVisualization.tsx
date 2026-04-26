import React from "react";
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  AreaChart,
  Area,
  PieChart,
  Pie,
  ScatterChart,
  Scatter,
  RadarChart,
  Radar,
  PolarAngleAxis,
  RadialBarChart,
  RadialBar,
  Treemap,
  Funnel,
  ComposedChart,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
  Cell,
} from "recharts";
import { ChartData } from "../types/chart";

interface ChartVisualizationProps {
  type: string;
  title: string;
  dataset: ChartData["dataset"];
  caption?: string;
}

export const ChartVisualization: React.FC<ChartVisualizationProps> = ({
  type,
  title,
  dataset,
  caption,
}) => {
  // Infer the numeric field name from the first object (skip "name")
  let metricKey: string | null = null;
  if (dataset && dataset.length > 0) {
    const firstItem = dataset[0];
    for (const key in firstItem) {
      if (
        key !== "name" &&
        typeof firstItem[key] === "number"
      ) {
        metricKey = key;
        break;
      }
    }
  }

  if (!metricKey) {
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
        Failed to render chart: no numeric fields found in dataset
      </div>
    );
  }

  const renderChart = () => {
    const chartProps = {
      data: dataset,
      margin: { top: 20, right: 30, left: 0, bottom: 20 },
    };

    switch (type) {
      case "bar":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <BarChart {...chartProps}>
              <CartesianGrid strokeDasharray="3 3" stroke="#2a2e36" />
              <XAxis dataKey="name" stroke="#8a8680" />
              <YAxis stroke="#8a8680" />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1d2027",
                  border: "1px solid #2a2e36",
                  color: "#e8e6e3",
                }}
              />
              <Legend />
              <Bar dataKey={metricKey} fill="#FFB800" />
            </BarChart>
          </ResponsiveContainer>
        );

      case "line":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <LineChart {...chartProps}>
              <CartesianGrid strokeDasharray="3 3" stroke="#2a2e36" />
              <XAxis dataKey="name" stroke="#8a8680" />
              <YAxis stroke="#8a8680" />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1d2027",
                  border: "1px solid #2a2e36",
                  color: "#e8e6e3",
                }}
              />
              <Legend />
              <Line type="monotone" dataKey={metricKey} stroke="#FFB800" />
            </LineChart>
          </ResponsiveContainer>
        );

      case "area":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <AreaChart {...chartProps}>
              <CartesianGrid strokeDasharray="3 3" stroke="#2a2e36" />
              <XAxis dataKey="name" stroke="#8a8680" />
              <YAxis stroke="#8a8680" />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1d2027",
                  border: "1px solid #2a2e36",
                  color: "#e8e6e3",
                }}
              />
              <Legend />
              <Area
                type="monotone"
                dataKey={metricKey}
                fill="#FFB800"
                stroke="#FFB800"
              />
            </AreaChart>
          </ResponsiveContainer>
        );

      case "composed":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <ComposedChart {...chartProps}>
              <CartesianGrid strokeDasharray="3 3" stroke="#2a2e36" />
              <XAxis dataKey="name" stroke="#8a8680" />
              <YAxis stroke="#8a8680" />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1d2027",
                  border: "1px solid #2a2e36",
                  color: "#e8e6e3",
                }}
              />
              <Legend />
              <Bar dataKey={metricKey} fill="#FFB800" />
              <Line type="monotone" dataKey={metricKey} stroke="#FFB800" />
            </ComposedChart>
          </ResponsiveContainer>
        );

      case "scatter":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <ScatterChart {...chartProps}>
              <CartesianGrid strokeDasharray="3 3" stroke="#2a2e36" />
              <XAxis dataKey="name" stroke="#8a8680" />
              <YAxis stroke="#8a8680" />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1d2027",
                  border: "1px solid #2a2e36",
                  color: "#e8e6e3",
                }}
              />
              <Legend />
              <Scatter dataKey={metricKey} fill="#FFB800" />
            </ScatterChart>
          </ResponsiveContainer>
        );

      case "pie":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <PieChart>
              <Pie
                data={dataset}
                dataKey={metricKey}
                nameKey="name"
                cx="50%"
                cy="50%"
                outerRadius={80}
                fill="#FFB800"
                label
              >
                {dataset.map((_entry, index) => (
                  <Cell key={`cell-${index}`} fill="#FFB800" />
                ))}
              </Pie>
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1d2027",
                  border: "1px solid #2a2e36",
                  color: "#e8e6e3",
                }}
              />
            </PieChart>
          </ResponsiveContainer>
        );

      case "radar":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <RadarChart {...chartProps}>
              <CartesianGrid strokeDasharray="3 3" stroke="#2a2e36" />
              <PolarAngleAxis dataKey="name" stroke="#8a8680" />
              <Radar
                name={metricKey}
                dataKey={metricKey}
                stroke="#FFB800"
                fill="#FFB800"
                fillOpacity={0.6}
              />
              <Legend />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1d2027",
                  border: "1px solid #2a2e36",
                  color: "#e8e6e3",
                }}
              />
            </RadarChart>
          </ResponsiveContainer>
        );

      case "radialBar":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <RadialBarChart {...chartProps}>
              <CartesianGrid strokeDasharray="3 3" stroke="#2a2e36" />
              <PolarAngleAxis dataKey="name" stroke="#8a8680" />
              <RadialBar
                name={metricKey}
                dataKey={metricKey}
                fill="#FFB800"
              />
              <Legend />
              <Tooltip
                contentStyle={{
                  backgroundColor: "#1d2027",
                  border: "1px solid #2a2e36",
                  color: "#e8e6e3",
                }}
              />
            </RadialBarChart>
          </ResponsiveContainer>
        );

      case "treemap":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <Treemap
              data={dataset}
              dataKey={metricKey}
              nameKey="name"
              fill="#FFB800"
              stroke="#2a2e36"
            />
          </ResponsiveContainer>
        );

      case "funnel":
        return (
          <ResponsiveContainer width="100%" height={300}>
            <Funnel data={dataset} dataKey={metricKey} nameKey="name">
              {dataset.map((_entry, index) => (
                <Cell key={`cell-${index}`} fill="#FFB800" />
              ))}
            </Funnel>
          </ResponsiveContainer>
        );

      default:
        return <div>Unknown chart type: {type}</div>;
    }
  };

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
      {renderChart()}
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
