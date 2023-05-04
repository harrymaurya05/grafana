// Code generated - EDITING IS FUTILE. DO NOT EDIT.
//
// Generated by:
//     public/app/plugins/gen.go
// Using jennies:
//     TSTypesJenny
//     PluginTSTypesJenny
//
// Run 'make gen-cue' from repository root to regenerate.

import * as common from '@grafana/schema';

export const PanelCfgModelVersion = Object.freeze([0, 0]);

export enum SeriesMapping {
  Auto = 'auto',
  Manual = 'manual',
}

export enum ScatterShow {
  Lines = 'lines',
  Points = 'points',
  PointsAndLines = 'points+lines',
}

export interface XYDimensionConfig {
  exclude?: Array<string>;
  frame: number;
  x?: string;
}

export const defaultXYDimensionConfig: Partial<XYDimensionConfig> = {
  exclude: [],
};

export interface ScatterFieldConfig extends common.HideableFieldConfig, common.AxisConfig {
  label?: common.VisibilityMode;
  labelValue?: common.TextDimensionConfig;
  lineColor?: common.ColorDimensionConfig;
  lineStyle?: common.LineStyle;
  lineWidth?: number;
  opacity?: number; // TODO: 0-1, default to 0.5
  pointColor?: common.ColorDimensionConfig;
  pointSize?: common.ScaleDimensionConfig;
  show?: ScatterShow;
  symbol?: common.ResourceDimensionConfig;
}

export const defaultScatterFieldConfig: Partial<ScatterFieldConfig> = {
  label: common.VisibilityMode.Auto,
  show: ScatterShow.Points,
};

export interface ScatterSeriesConfig extends ScatterFieldConfig {
  name?: string;
  x?: string;
  y?: string;
}

export interface PanelOptions extends common.OptionsWithLegend, common.OptionsWithTooltip {
  dims: XYDimensionConfig;
  series: Array<ScatterSeriesConfig>;
  seriesMapping?: SeriesMapping;
}

export const defaultPanelOptions: Partial<PanelOptions> = {
  series: [],
};
