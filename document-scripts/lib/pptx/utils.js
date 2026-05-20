const THEMES = {
  corporate: {
    background: 'FFFFFF',
    header: '1B3A6B',
    title: '1B3A6B',
    heading: '2E4057',
    body: '333333',
    muted: '666666',
  },
  dark: {
    background: '1E1E1E',
    header: '333333',
    title: 'FFFFFF',
    heading: 'E0E0E0',
    body: 'CCCCCC',
    muted: '888888',
  },
  light: {
    background: 'F8F9FA',
    header: 'E9ECEF',
    title: '212529',
    heading: '495057',
    body: '343A40',
    muted: '6C757D',
  },
};

function getTheme(themeName) {
  return THEMES[themeName] || THEMES.corporate;
}

module.exports = {
  getTheme,
};
