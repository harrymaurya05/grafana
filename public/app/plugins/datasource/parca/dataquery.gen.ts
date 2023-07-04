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

export type ParcaQueryType = ('metrics' | 'profile' | 'both');

export const defaultParcaQueryType: ParcaQueryType = 'both';

export interface Parca extends common.DataQuery {
  /**
   * Specifies the query label selectors.
   */
  labelSelector: string;
  /**
   * Specifies the type of profile to query.
   */
  profileTypeId: string;
}

export const defaultParca: Partial<Parca> = {
  labelSelector: '{}',
};
