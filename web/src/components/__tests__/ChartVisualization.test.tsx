import { render, screen } from '@testing-library/react';
import { ChartVisualization } from '../ChartVisualization';
import '@testing-library/jest-dom';

const sampleDataset = [
  { name: 'Q1', value: 1200 },
  { name: 'Q2', value: 1800 },
  { name: 'Q3', value: 2200 },
];

describe('ChartVisualization', () => {
  it('renders bar chart', () => {
    render(
      <ChartVisualization
        type="bar"
        title="Sales Chart"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Sales Chart')).toBeInTheDocument();
  });

  it('renders line chart', () => {
    render(
      <ChartVisualization
        type="line"
        title="Trend Chart"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Trend Chart')).toBeInTheDocument();
  });

  it('renders area chart', () => {
    render(
      <ChartVisualization
        type="area"
        title="Area Chart"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Area Chart')).toBeInTheDocument();
  });

  it('renders pie chart', () => {
    render(
      <ChartVisualization
        type="pie"
        title="Distribution"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Distribution')).toBeInTheDocument();
  });

  it('renders composed chart', () => {
    render(
      <ChartVisualization
        type="composed"
        title="Composed"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Composed')).toBeInTheDocument();
  });

  it('renders scatter chart', () => {
    render(
      <ChartVisualization
        type="scatter"
        title="Scatter"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Scatter')).toBeInTheDocument();
  });

  it('renders radar chart', () => {
    render(
      <ChartVisualization
        type="radar"
        title="Radar"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Radar')).toBeInTheDocument();
  });

  it('renders radialBar chart', () => {
    render(
      <ChartVisualization
        type="radialBar"
        title="Radial"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Radial')).toBeInTheDocument();
  });

  it('renders treemap', () => {
    render(
      <ChartVisualization
        type="treemap"
        title="Treemap"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Treemap')).toBeInTheDocument();
  });

  it('renders funnel chart', () => {
    render(
      <ChartVisualization
        type="funnel"
        title="Funnel"
        dataset={sampleDataset}
      />
    );
    expect(screen.getByText('Funnel')).toBeInTheDocument();
  });

  it('renders caption when provided', () => {
    render(
      <ChartVisualization
        type="bar"
        title="Chart"
        dataset={sampleDataset}
        caption="Sample data"
      />
    );
    expect(screen.getByText('Sample data')).toBeInTheDocument();
  });

  it('handles empty dataset gracefully', () => {
    render(
      <ChartVisualization
        type="bar"
        title="Chart"
        dataset={[]}
      />
    );
    expect(screen.getByText(/no numeric fields/i)).toBeInTheDocument();
  });

  it('renders error when dataset has no numeric fields', () => {
    const badDataset = [{ name: 'A' }, { name: 'B' }];
    render(
      <ChartVisualization
        type="bar"
        title="Chart"
        dataset={badDataset as any}
      />
    );
    expect(screen.getByText(/no numeric fields/i)).toBeInTheDocument();
  });
});
