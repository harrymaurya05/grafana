import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { setupServer } from 'msw/node';
import { default as React } from 'react';
import { FormProvider, useForm } from 'react-hook-form';
import { Provider } from 'react-redux';

import { setBackendSrv } from '@grafana/runtime';
import { backendSrv } from 'app/core/services/backend_srv';
import { configureStore } from 'app/store/configureStore';

import 'whatwg-fetch';
import { TemplatesPreviewResponse } from '../../api/templateApi';
import { mockPreviewTemplateResponse, mockPreviewTemplateResponseRejected } from '../../mocks/templatesApi';

import { defaults, PREVIEW_NOT_AVAILABLE, TemplateFormValues, TemplatePreview } from './TemplateForm';

const mockSetPayloadFormatError = jest.fn();
const getProviderWraper = () => {
  return function Wrapper({ children }: React.PropsWithChildren<{}>) {
    const store = configureStore();
    const formApi = useForm<TemplateFormValues>({ defaultValues: defaults });
    return (
      <Provider store={store}>
        <FormProvider {...formApi}>{children}</FormProvider>
      </Provider>
    );
  };
};

const server = setupServer();

beforeAll(() => {
  setBackendSrv(backendSrv);
  server.listen({ onUnhandledRequest: 'error' });
});

beforeEach(() => {
  server.resetHandlers();
});

afterAll(() => {
  server.close();
});

describe('TemplatePreview component', () => {
  it('Should render error if payload has wrong format', async () => {
    render(
      <TemplatePreview
        payload={'whatever'}
        payloadFormatError={'ERROR IN JSON FORMAT'}
        setPayloadFormatError={mockSetPayloadFormatError}
        templateName="potato"
      />,
      { wrapper: getProviderWraper() }
    );
    await waitFor(() => {
      expect(screen.getByTestId('payloadJSON')).toHaveTextContent('ERROR IN JSON FORMAT');
    });
  });

  it('Should render error if payload has wrong format after clicking preview button', async () => {
    render(
      <TemplatePreview
        payload={'whatever'}
        payloadFormatError={'ERROR IN JSON FORMAT'}
        setPayloadFormatError={mockSetPayloadFormatError}
        templateName="potato"
      />,
      { wrapper: getProviderWraper() }
    );
    const button = screen.getByRole('button', {
      name: /preview/i,
    });

    within(button).getByText(/preview/i);
    await userEvent.click(within(button).getByText(/preview/i));
    await waitFor(() => {
      expect(screen.getByTestId('payloadJSON')).toHaveTextContent('ERROR IN JSON FORMAT');
    });
  });

  it('Should render error in preview response , after clicking preview, if payload has correct format after clicking preview button, but preview request has been rejected', async () => {
    mockPreviewTemplateResponseRejected(server);
    render(
      <TemplatePreview
        payload={'{"a":"b"}'}
        payloadFormatError={null}
        setPayloadFormatError={mockSetPayloadFormatError}
        templateName="potato"
      />,
      { wrapper: getProviderWraper() }
    );
    const button = screen.getByRole('button', {
      name: /preview/i,
    });

    within(button).getByText(/preview/i);
    await userEvent.click(within(button).getByText(/preview/i));
    await waitFor(() => {
      expect(screen.getByTestId('payloadJSON')).toHaveTextContent(PREVIEW_NOT_AVAILABLE);
    });
  });

  it('Should render preview response , after clicking preview, if payload has correct format after clicking preview button', async () => {
    const response: TemplatesPreviewResponse = {
      results: [
        { name: 'template1', text: 'This is the template result bla bla bla' },
        { name: 'template2', text: 'This is the template2 result bla bla bla' },
      ],
    };
    mockPreviewTemplateResponse(server, response);
    render(
      <TemplatePreview
        payload={'{"a":"b"}'}
        payloadFormatError={null}
        setPayloadFormatError={mockSetPayloadFormatError}
        templateName="potato"
      />,
      { wrapper: getProviderWraper() }
    );
    const button = screen.getByRole('button', {
      name: /preview/i,
    });

    within(button).getByText(/preview/i);
    await userEvent.click(within(button).getByText(/preview/i));
    await waitFor(() => {
      expect(screen.getByTestId('payloadJSON')).toHaveTextContent(
        'Preview for template1: This is the template result bla bla bla Preview for template2: This is the template2 result bla bla bla'
      );
    });
  });
  it('Should render preview response with some errors, after clicking preview, if payload has correct format after clicking preview button', async () => {
    const response: TemplatesPreviewResponse = {
      results: [{ name: 'template1', text: 'This is the template result bla bla bla' }],
      errors: [
        { name: 'template2', message: 'Unexpected "{" in operand', kind: 'kind_of_error' },
        { name: 'template3', kind: 'kind_of_error', message: 'Unexpected "{" in operand' },
      ],
    };
    mockPreviewTemplateResponse(server, response);
    render(
      <TemplatePreview
        payload={'{"a":"b"}'}
        payloadFormatError={null}
        setPayloadFormatError={mockSetPayloadFormatError}
        templateName="potato"
      />,
      { wrapper: getProviderWraper() }
    );
    const button = screen.getByRole('button', {
      name: /preview/i,
    });

    within(button).getByText(/preview/i);
    await userEvent.click(within(button).getByText(/preview/i));
    await waitFor(() => {
      expect(screen.getByTestId('payloadJSON')).toHaveTextContent(
        'Preview for template1: This is the template result bla bla bla ERROR in template2: kind_of_error Unexpected "{" in operand ERROR in template3: kind_of_error Unexpected "{" in operand'
      );
    });
  });
});
