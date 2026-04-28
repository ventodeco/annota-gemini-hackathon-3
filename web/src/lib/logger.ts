type LogLevel = 'debug' | 'info' | 'warn' | 'error';

const LOG_LEVELS: LogLevel[] = ['debug', 'info', 'warn', 'error'];

function isLogLevel(value: string): value is LogLevel {
  return LOG_LEVELS.some((level) => level === value);
}

export function resolveLogLevel(value: unknown): LogLevel {
  return typeof value === 'string' && isLogLevel(value) ? value : 'info';
}

interface LogEntry {
  timestamp: string;
  level: LogLevel;
  message: string;
  context?: Record<string, unknown>;
  error?: {
    name: string;
    message: string;
    stack?: string;
  };
  requestId?: string;
  userId?: string;
}

interface LoggerConfig {
  level: LogLevel;
  enableConsole: boolean;
  enableRemote: boolean;
  remoteEndpoint?: string;
}

const defaultConfig: LoggerConfig = {
  level: resolveLogLevel(import.meta.env.VITE_LOG_LEVEL),
  enableConsole: true,
  enableRemote: false,
  remoteEndpoint: import.meta.env.VITE_LOG_ENDPOINT,
};

class Logger {
  private config: LoggerConfig;
  private requestId: string | null = null;
  private userId: string | null = null;
  private context: Record<string, unknown> = {};

  constructor(config: LoggerConfig = defaultConfig) {
    this.config = { ...defaultConfig, ...config };
  }

  setLevel(level: LogLevel): void {
    this.config.level = level;
  }

  setRequestId(requestId: string): void {
    this.requestId = requestId;
  }

  setUserId(userId: string): void {
    this.userId = userId;
  }

  addContext(key: string, value: unknown): void {
    this.context[key] = value;
  }

  clearContext(): void {
    this.context = {};
  }

  private shouldLog(level: LogLevel): boolean {
    return LOG_LEVELS.indexOf(level) >= LOG_LEVELS.indexOf(this.config.level);
  }

  private formatMessage(_level: LogLevel, message: string, ...args: unknown[]): string {
    if (args.length > 0) {
      return `${message} ${args.map(arg => JSON.stringify(arg)).join(' ')}`;
    }
    return message;
  }

  private createEntry(
    level: LogLevel,
    message: string,
    error?: Error,
    context?: Record<string, unknown>
  ): LogEntry {
    const entry: LogEntry = {
      timestamp: new Date().toISOString(),
      level,
      message,
      context: { ...this.context, ...context },
      requestId: this.requestId || undefined,
      userId: this.userId || undefined,
    };

    if (error) {
      entry.error = {
        name: error.name || 'Error',
        message: error.message,
        stack: error.stack,
      };
    }

    return entry;
  }

  private logToConsole(_level: LogLevel, entry: LogEntry): void {
    const { timestamp, level: logLevel, message, context, error } = entry;
    const prefix = `[${timestamp}] [${logLevel.toUpperCase()}]`;

    switch (logLevel) {
      case 'debug':
        console.debug(prefix, message, context, error || '');
        break;
      case 'info':
        console.info(prefix, message, context, error || '');
        break;
      case 'warn':
        console.warn(prefix, message, context, error || '');
        break;
      case 'error':
        console.error(prefix, message, context, error || '');
        break;
    }
  }

  private async sendToRemote(entry: LogEntry): Promise<void> {
    if (!this.config.enableRemote || !this.config.remoteEndpoint) {
      return;
    }

    try {
      await fetch(this.config.remoteEndpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(entry),
      });
    } catch (error) {
      console.error('Failed to send log to remote:', error);
    }
  }

  private log(
    level: LogLevel,
    message: string,
    error?: Error,
    context?: Record<string, unknown>
  ): void {
    if (!this.shouldLog(level)) {
      return;
    }

    const entry = this.createEntry(level, message, error, context);

    if (this.config.enableConsole) {
      this.logToConsole(level, entry);
    }

    if (this.config.enableRemote) {
      this.sendToRemote(entry);
    }
  }

  debug(message: string, context?: Record<string, unknown>): void {
    this.log('debug', message, undefined, context);
  }

  debugf(format: string, ...args: unknown[]): void {
    const message = this.formatMessage('debug', format, ...args);
    this.log('debug', message);
  }

  info(message: string, context?: Record<string, unknown>): void {
    this.log('info', message, undefined, context);
  }

  infof(format: string, ...args: unknown[]): void {
    const message = this.formatMessage('info', format, ...args);
    this.log('info', message);
  }

  warn(message: string, context?: Record<string, unknown>): void {
    this.log('warn', message, undefined, context);
  }

  warnf(format: string, ...args: unknown[]): void {
    const message = this.formatMessage('warn', format, ...args);
    this.log('warn', message);
  }

  error(message: string, error?: Error, context?: Record<string, unknown>): void {
    this.log('error', message, error, context);
  }

  errorf(format: string, ...args: unknown[]): void {
    const message = this.formatMessage('error', format, ...args);
    this.log('error', message);
  }

  errorWithContext(message: string, context: Record<string, unknown>, error?: Error): void {
    this.log('error', message, error, context);
  }

  apiCall(
    method: string,
    url: string,
    status: number,
    duration: number,
    error?: Error
  ): void {
    const context = {
      api: {
        method,
        url,
        status,
        duration_ms: duration,
      },
    };

    if (error || (status >= 400 && status < 600)) {
      this.log('error', `API call failed: ${method} ${url}`, error, context);
    } else if (status >= 300 && status < 400) {
      this.log('warn', `API redirect: ${method} ${url}`, undefined, context);
    } else {
      this.log('info', `API call success: ${method} ${url}`, undefined, context);
    }
  }

  componentMount(componentName: string, props?: Record<string, unknown>): void {
    this.debug(`Component mounted: ${componentName}`, { component: componentName, props });
  }

  componentUnmount(componentName: string): void {
    this.debug(`Component unmounted: ${componentName}`, { component: componentName });
  }

  userAction(action: string, details?: Record<string, unknown>): void {
    this.info(`User action: ${action}`, { action, ...details });
  }

  navigation(from: string, to: string): void {
    this.info(`Navigation: ${from} -> ${to}`, { from, to });
  }
}

export const logger = new Logger();

export function createLogger(config: Partial<LoggerConfig>): Logger {
  return new Logger({ ...defaultConfig, ...config });
}

export type { LogLevel, LogEntry, LoggerConfig };
