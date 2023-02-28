import { isNumber } from 'lodash';
import React, { PureComponent } from 'react';

import {
  DisplayValueAlignmentFactors,
  FieldDisplay,
  FieldType,
  getDisplayValueAlignmentFactors,
  getFieldDisplayValues,
  NumericRange,
  PanelProps,
} from '@grafana/data';
import { findNumericFieldMinMax } from '@grafana/data/src/field/fieldOverrides';
import { BigValueTextMode, BigValueGraphMode } from '@grafana/schema';
import { BigValue, DataLinksContextMenu, VizRepeater, VizRepeaterRenderValueProps } from '@grafana/ui';
import { DataLinksContextMenuApi } from '@grafana/ui/src/components/DataLinks/DataLinksContextMenu';
import { config } from 'app/core/config';

import { PanelOptions } from './panelcfg.gen';

export class StatPanel extends PureComponent<PanelProps<PanelOptions>> {
  renderComponent = (
    valueProps: VizRepeaterRenderValueProps<FieldDisplay, DisplayValueAlignmentFactors>,
    menuProps: DataLinksContextMenuApi
  ): JSX.Element => {
    const { timeRange, options } = this.props;
    // console.log('🚀 ~ file: StatPanel.tsx:27 ~ StatPanel ~ this.props:', this.props);
    // console.log(this.props.fieldConfig.defaults.custom, 'custome config');
    const { value, alignmentFactors, width, height, count, orientation } = valueProps;
    const { openMenu, targetClassName } = menuProps;
    let sparkline = value.sparkline;
    if (sparkline) {
      sparkline.timeRange = timeRange;
    }

    return (
      <BigValue
        value={value.display}
        count={count}
        sparkline={sparkline}
        colorMode={options.colorMode}
        graphMode={options.graphMode}
        justifyMode={options.justifyMode}
        textMode={this.getTextMode()}
        alignmentFactors={alignmentFactors}
        parentOrientation={orientation}
        text={options.text}
        width={width}
        height={height}
        theme={config.theme2}
        onClick={openMenu}
        className={targetClassName}
      />
    );
  };

  getTextMode() {
    const { options, fieldConfig, title } = this.props;

    // If we have manually set displayName or panel title switch text mode to value and name
    if (options.textMode === BigValueTextMode.Auto && (fieldConfig.defaults.displayName || !title)) {
      return BigValueTextMode.ValueAndName;
    }

    return options.textMode;
  }

  renderValue = (valueProps: VizRepeaterRenderValueProps<FieldDisplay, DisplayValueAlignmentFactors>): JSX.Element => {
    const { value } = valueProps;
    const { getLinks, hasLinks } = value;

    if (hasLinks && getLinks) {
      return (
        <DataLinksContextMenu links={getLinks}>
          {(api) => {
            return this.renderComponent(valueProps, api);
          }}
        </DataLinksContextMenu>
      );
    }

    return this.renderComponent(valueProps, {});
  };

  getValues = (): FieldDisplay[] => {
    const { data, options, replaceVariables, fieldConfig, timeZone } = this.props;
    // Test if there is a custom unit to prepend
    const customPrefix = fieldConfig.defaults?.custom?.prependUnit;

    // console.log(fieldConfig, 'fieldconfig');

    let globalRange: NumericRange | undefined = undefined;

    for (let frame of data.series) {
      for (let field of frame.fields) {
        let { config } = field;
        // console.log(config, 'config');
        // mostly copied from fieldOverrides, since they are skipped during streaming
        // Set the Min/Max value automatically
        if (field.type === FieldType.number) {
          if (field.state?.range) {
            continue;
          }
          if (!globalRange && (!isNumber(config.min) || !isNumber(config.max))) {
            globalRange = findNumericFieldMinMax(data.series);
          }
          const min = config.min ?? globalRange!.min;
          const max = config.max ?? globalRange!.max;
          field.state = field.state ?? {};
          field.state.range = { min, max, delta: max! - min! };
        }
      }
    }

    const fieldDisplayValues = getFieldDisplayValues({
      fieldConfig,
      reduceOptions: options.reduceOptions,
      replaceVariables,
      theme: config.theme2,
      data: data.series,
      sparkline: options.graphMode !== BigValueGraphMode.None,
      timeZone,
    });

    const updatedVals = this.formatValueForCustomPrefix(fieldDisplayValues, customPrefix);

    console.log(updatedVals);

    return updatedVals;
  };

  formatValueForCustomPrefix(fieldValues: FieldDisplay[], customPrefix: string): FieldDisplay[] {
    return fieldValues.map((fieldValue) => {
      const { fieldType, display } = fieldValue;
      if (fieldType === FieldType.number) {
        const previousPrefix = display.prefix ?? '';
        const updatedPrefix = customPrefix + previousPrefix;
        const updatedDisplay = { ...display, prefix: updatedPrefix };
        return { ...fieldValue, display: updatedDisplay };
      }
      return fieldValue;
    });
  }

  render() {
    const { height, options, width, data, renderCounter } = this.props;

    return (
      <VizRepeater
        getValues={this.getValues}
        getAlignmentFactors={getDisplayValueAlignmentFactors}
        renderValue={this.renderValue}
        width={width}
        height={height}
        source={data}
        itemSpacing={3}
        renderCounter={renderCounter}
        autoGrid={true}
        orientation={options.orientation}
      />
    );
  }
}

export const getOverwriteSymbols = () => {
  return {
    name: 'Custom Prefix Symbols',
    formats: [
      // { name: 'Percent Increase (\u2191_%)', id: 'percentincrease', fn: toIncreasingPercent },
      // { name: 'Percent Decrease (\u2193_%)', id: 'percentdecrease', fn: toDecreasingPercent },
      { name: 'Remove Custom Prefix', id: 'remove' },
      { name: 'Less than (<)', id: 'lessThan' },
      { name: 'Greater than (>)', id: 'greaterThan' },
      { name: 'Approximately (~)', id: 'approximately' },
      { name: 'Fiscal quarter (FQ)', id: 'fiscalQuarter' },
      { name: 'Quarter (Qtr)', id: 'quarter' },
      { name: 'Fiscal year (FY)', id: 'fiscalYear' },
      { name: 'Delta (\u0394)', id: 'delta' },
      { name: 'Mean (\u00B5)', id: 'mean' },
    ],
  };
};
