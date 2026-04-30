/**
 * Chart types and data structures for the chart visualization feature.
 */

export type ChartType =
  | 'area'
  | 'bar'
  | 'line'
  | 'composed'
  | 'scatter'
  | 'pie'
  | 'radar'
  | 'radialBar'
  | 'treemap'
  | 'funnel';

export const VALID_CHART_TYPES: ChartType[] = [
  'area',
  'bar',
  'line',
  'composed',
  'scatter',
  'pie',
  'radar',
  'radialBar',
  'treemap',
  'funnel',
];

export interface DataPoint {
  name: string;
  [key: string]: string | number;
}

export interface ChartData {
  type: ChartType;
  title: string;
  dataset: DataPoint[];
  caption?: string;
}

export interface ChartToolResponse {
  success: boolean;
  type?: ChartType;
  title?: string;
  dataset?: string; // JSON string
  message: string;
}
