'use client';

import { FormEvent, useEffect, useMemo, useState } from 'react';
import { motion, AnimatePresence } from 'framer-motion';

const apiUrl = (process.env.NEXT_PUBLIC_API_URL ?? 'http://127.0.0.1:8080').replace(/\/$/, '');
const tokenKey = 'launcher.admin.token';

type AuthUser = {
  id: string;
  login: string;
  providerUuid: string;
  role: string;
};

type Profile = {
  id: string;
  name: string;
  slug: string;
  description: string;
  loader: string;
  gameVersion: string;
  loaderVersion: string;
  javaVersion: number;
  jvmArgs: string;
  iconUrl: string;
  javaPathWindows: string;
  javaPathLinux: string;
  javaPathMacos: string;
  launchCommandWindows: string;
  launchCommandLinux: string;
  launchCommandMacos: string;
  preservePaths: string[];
  manifestVersion: number;
  manifestUpdatedAt?: string;
  isActive: boolean;
  fileCount: number;
  totalSize: number;
  clientPrepared: boolean;
  clientStatus: string;
};

type ProfileForm = Omit<
  Profile,
  'id' | 'manifestVersion' | 'manifestUpdatedAt' | 'fileCount' | 'totalSize' | 'clientPrepared' | 'clientStatus'
>;

type LoaderCatalog = {
  minecraftVersions: string[];
  loaders: LoaderOption[];
};

type LoaderOption = {
  id: string;
  label: string;
  javaVersion: number;
  requiresVersion: boolean;
  versions: LoaderVersion[];
};

type LoaderVersion = {
  value: string;
  label: string;
  stable: boolean;
};

type LoginForm = {
  login: string;
  password: string;
  totp: string;
};

type ApiError = {
  message?: string;
};

type EditorSection = 'main' | 'loader' | 'java' | 'launch' | 'security' | 'build';

const defaultPreservePaths = [
  'saves/',
  'resourcepacks/',
  'shaderpacks/',
  'screenshots/',
  'logs/',
  'crash-reports/',
  'options.txt',
  'optionsof.txt',
  'servers.dat'
];

const emptyProfile: ProfileForm = {
  name: '',
  slug: '',
  description: '',
  loader: 'vanilla',
  gameVersion: '1.21.1',
  loaderVersion: '',
  javaVersion: 21,
  jvmArgs: '',
  iconUrl: '',
  javaPathWindows: 'runtime/windows-x64/bin/java.exe',
  javaPathLinux: 'runtime/linux/bin/java',
  javaPathMacos: 'runtime/mac-os/jre.bundle/Contents/Home/bin/java',
  launchCommandWindows: '{java} {jvm_args} -jar client.jar --username {login} --uuid {uuid} --accessToken {access_token} --gameDir {game_dir}',
  launchCommandLinux: '{java} {jvm_args} -jar client.jar --username {login} --uuid {uuid} --accessToken {access_token} --gameDir {game_dir}',
  launchCommandMacos: '{java} {jvm_args} -jar client.jar --username {login} --uuid {uuid} --accessToken {access_token} --gameDir {game_dir}',
  preservePaths: [...defaultPreservePaths],
  isActive: true
};

const editorSections: Array<{ id: EditorSection; label: string }> = [
  { id: 'main', label: 'Основное' },
  { id: 'loader', label: 'Загрузчик' },
  { id: 'java', label: 'Java' },
  { id: 'launch', label: 'Запуск' },
  { id: 'security', label: 'Защита' },
  { id: 'build', label: 'Сборка' }
];

export function ProfileAdmin() {
  const [token, setToken] = useState('');
  const [user, setUser] = useState<AuthUser | null>(null);
  const [loginForm, setLoginForm] = useState<LoginForm>({ login: '', password: '', totp: '' });
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [form, setForm] = useState<ProfileForm>(emptyProfile);
  const [message, setMessage] = useState('Ожидание авторизации');
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [scanId, setScanId] = useState<string | null>(null);
  const [prepareId, setPrepareId] = useState<string | null>(null);
  const [folderEdited, setFolderEdited] = useState(false);
  const [editorSection, setEditorSection] = useState<EditorSection>('main');
  const [loaderCatalog, setLoaderCatalog] = useState<LoaderCatalog>({ minecraftVersions: [], loaders: [] });
  const [isLoadingLoaders, setIsLoadingLoaders] = useState(false);

  const selectedProfile = useMemo(
    () => profiles.find((profile) => profile.id === selectedId) ?? null,
    [profiles, selectedId]
  );
  const isPreparingSelected = selectedProfile ? prepareId === selectedProfile.id : false;
  const selectedClientReady = Boolean(selectedProfile?.clientPrepared);

  const selectedLoader = useMemo(
    () => loaderCatalog.loaders.find((loader) => loader.id === form.loader),
    [loaderCatalog.loaders, form.loader]
  );

  useEffect(() => {
    const savedToken = localStorage.getItem(tokenKey);
    if (savedToken) {
      setToken(savedToken);
      void restoreSession(savedToken);
    }
  }, []);

  useEffect(() => {
    if (!selectedProfile) {
      return;
    }
    setForm(profileToForm(selectedProfile));
    setFolderEdited(true);
  }, [selectedProfile]);

  useEffect(() => {
    if (!user || !token) {
      return;
    }
    const timeout = window.setTimeout(() => {
      void loadLoaderOptions(form.gameVersion);
    }, 250);
    return () => window.clearTimeout(timeout);
  }, [form.gameVersion, token, user]);

  useEffect(() => {
    if (loaderCatalog.loaders.length === 0) {
      return;
    }
    setForm((current) => {
      const loader = loaderCatalog.loaders.find((item) => item.id === current.loader);
      if (!loader) {
        const fallback = loaderCatalog.loaders.find((item) => item.id === 'vanilla') ?? loaderCatalog.loaders[0];
        return {
          ...current,
          loader: fallback.id,
          javaVersion: fallback.javaVersion,
          loaderVersion: fallback.requiresVersion ? fallback.versions[0]?.value ?? '' : ''
        };
      }
      if (!loader.requiresVersion) {
        return current.loaderVersion ? { ...current, loaderVersion: '' } : current;
      }
      const latestVersion = loader.versions[0]?.value ?? '';
      if (!latestVersion || loader.versions.some((version) => version.value === current.loaderVersion)) {
        return current;
      }
      return {
        ...current,
        javaVersion: loader.javaVersion,
        loaderVersion: latestVersion
      };
    });
  }, [loaderCatalog.loaders]);

  async function restoreSession(savedToken: string) {
    setIsLoading(true);
    try {
      const currentUser = await request<AuthUser>('/api/auth/me', { token: savedToken });
      if (currentUser.role !== 'admin') {
        throw new Error('Аккаунт не является администратором');
      }
      setUser(currentUser);
      setMessage(`Администратор: ${currentUser.login}`);
      await loadProfiles(savedToken);
      await loadLoaderOptions(emptyProfile.gameVersion, savedToken);
    } catch (error) {
      localStorage.removeItem(tokenKey);
      setToken('');
      setUser(null);
      setMessage(errorMessage(error));
    } finally {
      setIsLoading(false);
    }
  }

  async function handleLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsLoading(true);
    try {
      const response = await request<{ token: string; user: AuthUser }>('/api/auth/login', {
        method: 'POST',
        body: {
          login: loginForm.login,
          password: loginForm.password,
          totp: loginForm.totp || undefined
        }
      });
      if (response.user.role !== 'admin') {
        throw new Error('Этот аккаунт вошёл, но не имеет роли admin');
      }
      localStorage.setItem(tokenKey, response.token);
      setToken(response.token);
      setUser(response.user);
      setLoginForm({ login: response.user.login, password: '', totp: '' });
      setMessage(`Администратор: ${response.user.login}`);
      await loadProfiles(response.token);
      await loadLoaderOptions(emptyProfile.gameVersion, response.token);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setIsLoading(false);
    }
  }

  async function loadProfiles(activeToken = token) {
    const data = await request<Profile[]>('/api/admin/profiles', { token: activeToken });
    setProfiles(data);
    if (data.length > 0 && !selectedId) {
      setSelectedId(data[0].id);
      setForm(profileToForm(data[0]));
      setFolderEdited(true);
    }
    if (data.length === 0) {
      startNewProfile();
    }
  }

  async function loadLoaderOptions(gameVersion: string, activeToken = token) {
    if (!activeToken) {
      return;
    }
    setIsLoadingLoaders(true);
    try {
      const data = await request<LoaderCatalog>(
        `/api/admin/profiles/loader-options?gameVersion=${encodeURIComponent(gameVersion)}`,
        { token: activeToken }
      );
      setLoaderCatalog(data);
    } catch {
      setLoaderCatalog({ minecraftVersions: [], loaders: [] });
    } finally {
      setIsLoadingLoaders(false);
    }
  }

  async function saveProfile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSaving(true);
    try {
      const payload = normalizeProfileBeforeSave(form);
      const profile = selectedId
        ? await request<Profile>(`/api/admin/profiles/${selectedId}`, {
            method: 'PATCH',
            token,
            body: payload
          })
        : await request<Profile>('/api/admin/profiles', {
            method: 'POST',
            token,
            body: payload
          });
      await loadProfiles();
      setSelectedId(profile.id);
      setFolderEdited(true);
      setMessage(`Профиль сохранён: ${profile.name}`);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setIsSaving(false);
    }
  }

  async function scanProfile(profile: Profile) {
    setScanId(profile.id);
    setEditorSection('build');
    try {
      const result = await request<{ fileCount: number; totalSize: number }>(
        `/api/admin/profiles/${profile.id}/scan`,
        { method: 'POST', token }
      );
      await loadProfiles();
      setMessage(`Manifest собран: ${result.fileCount} файлов, ${formatBytes(result.totalSize)}`);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setScanId(null);
    }
  }

  async function prepareClient(profile: Profile) {
    setPrepareId(profile.id);
    setEditorSection('build');
    setMessage(`Файлы клиента скачиваются для профиля «${profile.name}». Это может занять несколько минут.`);
    try {
      const result = await request<{ fileCount: number; totalSize: number; downloaded: number; message: string }>(
        `/api/admin/profiles/${profile.id}/prepare-client`,
        { method: 'POST', token }
      );
      await loadProfiles();
      setMessage(
        `${result.message} Скачано/обновлено: ${result.downloaded}; manifest: ${result.fileCount} файлов, ${formatBytes(result.totalSize)}`
      );
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setPrepareId(null);
    }
  }

  async function deleteProfile(profile: Profile) {
    setIsLoading(true);
    try {
      await request(`/api/admin/profiles/${profile.id}`, { method: 'DELETE', token });
      startNewProfile();
      await loadProfiles();
      setMessage(`Профиль удалён из БД: ${profile.name}`);
    } catch (error) {
      setMessage(errorMessage(error));
    } finally {
      setIsLoading(false);
    }
  }

  function logout() {
    localStorage.removeItem(tokenKey);
    setToken('');
    setUser(null);
    setProfiles([]);
    startNewProfile();
    setMessage('Сессия dashboard завершена');
  }

  function startNewProfile() {
    setSelectedId(null);
    setForm({ ...emptyProfile, preservePaths: [...defaultPreservePaths] });
    setFolderEdited(false);
    setEditorSection('main');
  }

  function updateName(name: string) {
    setForm((current) => ({
      ...current,
      name,
      slug: folderEdited ? current.slug : folderNameFromTitle(name)
    }));
  }

  function updateLoader(loaderId: string) {
    const loader = loaderCatalog.loaders.find((item) => item.id === loaderId);
    setForm((current) => ({
      ...current,
      loader: loaderId,
      javaVersion: loader?.javaVersion ?? current.javaVersion,
      loaderVersion: loader?.requiresVersion ? loader.versions[0]?.value ?? current.loaderVersion : ''
    }));
  }

  function updateGameVersion(gameVersion: string) {
    setForm((current) => ({
      ...current,
      gameVersion,
      javaVersion: javaVersionForMinecraft(gameVersion),
      loaderVersion: ''
    }));
  }

  if (!user) {
    return (
      <section className="admin-panel glass">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Admin auth</p>
            <h2>Вход администратора</h2>
          </div>
          <span className="env-pill">{isLoading ? 'Проверка' : 'JWT'}</span>
        </div>
        <form className="admin-form compact" onSubmit={(event) => void handleLogin(event)}>
          <label>
            <span>Логин</span>
            <input
              value={loginForm.login}
              onChange={(event) => setLoginForm({ ...loginForm, login: event.target.value })}
              autoComplete="username"
            />
          </label>
          <label>
            <span>Пароль</span>
            <input
              value={loginForm.password}
              onChange={(event) => setLoginForm({ ...loginForm, password: event.target.value })}
              type="password"
              autoComplete="current-password"
            />
          </label>
          <label>
            <span>2FA</span>
            <input
              value={loginForm.totp}
              onChange={(event) => setLoginForm({ ...loginForm, totp: event.target.value })}
              inputMode="numeric"
            />
          </label>
          <button className="primary-button" type="submit" disabled={isLoading}>
            <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>login</span>
            Войти
          </button>
        </form>
        <p className="panel-message">{message}</p>
      </section>
    );
  }

  return (
    <section className="profile-admin-grid" id="profiles">
      <section className="admin-panel glass">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">Профили</p>
            <h2>Проектные сборки</h2>
          </div>
          <button className="icon-button" type="button" onClick={() => void loadProfiles()} title="Обновить">
            <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>refresh</span>
          </button>
        </div>

        <div className="flow-steps">
          <span className="done">1. Профиль</span>
          <span className={selectedClientReady ? 'done' : ''}>2. Клиент</span>
          <span className={selectedClientReady && (selectedProfile?.fileCount ?? 0) > 0 ? 'done' : ''}>3. Manifest</span>
        </div>

        <div className="profile-list">
          {profiles.map((profile) => (
            <motion.article
              initial={{ opacity: 0, scale: 0.95 }}
              animate={{ opacity: 1, scale: 1 }}
              whileHover={{ scale: 1.01 }}
              transition={{ duration: 0.2 }}
              className={profile.id === selectedId ? 'profile-row active' : 'profile-row'}
              key={profile.id}
            >
              <button type="button" onClick={() => setSelectedId(profile.id)}>
                <span>
                  <strong>{profile.name}</strong>
                  <small>Папка: {profile.slug}</small>
                </span>
                <span className={profile.isActive ? 'status-pill live' : 'status-pill'}>
                  {profile.isActive ? 'active' : 'off'}
                </span>
              </button>
              <div className="profile-row-meta">
                <span>{profile.gameVersion}</span>
                <span>{loaderLabel(profile.loader, loaderCatalog)}</span>
                <span>{profile.loaderVersion || 'без версии'}</span>
                <span>{profile.fileCount} файлов</span>
                <span>{formatBytes(profile.totalSize)}</span>
              </div>
              <div className="profile-state-line">
                <span className={prepareId === profile.id ? 'state-chip downloading' : profile.clientPrepared ? 'state-chip ready' : 'state-chip'}>
                  {prepareId === profile.id
                    ? 'Файлы скачиваются'
                    : profile.clientPrepared
                      ? 'Клиент готов'
                      : 'Клиент не подготовлен'}
                </span>
              </div>
              <div className="profile-actions">
                <button type="button" onClick={() => void prepareClient(profile)} disabled={prepareId === profile.id}>
                  <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 15 }}>download</span>
                  {prepareId === profile.id ? 'Скачиваем' : 'Клиент'}
                </button>
                <button
                  type="button"
                  onClick={() => void scanProfile(profile)}
                  disabled={!profile.clientPrepared || scanId === profile.id}
                >
                  <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 15 }}>build</span>
                  Собрать
                </button>
                <button type="button" onClick={() => void deleteProfile(profile)}>
                  <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 15 }}>delete</span>
                  Удалить
                </button>
              </div>
            </motion.article>
          ))}
        </div>

        <motion.button whileHover={{ scale: 1.02 }} whileTap={{ scale: 0.95 }} className="secondary-button" type="button" onClick={startNewProfile}>
          <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>add</span>
          Новый профиль
        </motion.button>
        <p className="panel-message">{message}</p>
        <button className="text-button" type="button" onClick={logout}>
          Выйти из dashboard
        </button>
      </section>

      <section className="admin-panel glass">
        <div className="panel-heading">
          <div>
            <p className="eyebrow">{selectedId ? 'Редактирование' : 'Создание'}</p>
            <h2>{selectedProfile?.name ?? 'Новый профиль'}</h2>
          </div>
          <span className="env-pill">manifest v{selectedProfile?.manifestVersion ?? 0}</span>
        </div>

        <div className="wizard-tabs" role="tablist" aria-label="Profile sections">
          {editorSections.map((section) => (
            <motion.button
              whileTap={{ scale: 0.95 }}
              className={editorSection === section.id ? 'active' : ''}
              key={section.id}
              type="button"
              onClick={() => setEditorSection(section.id)}
            >
              {section.label}
            </motion.button>
          ))}
        </div>

        <form className="admin-form profile-wizard" onSubmit={(event) => void saveProfile(event)}>
          <AnimatePresence mode="wait">
          {editorSection === 'main' && (
            <motion.section className="wizard-section" key={editorSection} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
              <div className="form-grid two">
                <TextField label="Название профиля" value={form.name} onChange={updateName} />
                <label>
                  <span>Папка профиля</span>
                  <input
                    value={form.slug}
                    onChange={(event) => {
                      setFolderEdited(true);
                      setForm({ ...form, slug: folderNameFromTitle(event.target.value) });
                    }}
                    placeholder="project-survival"
                  />
                </label>
              </div>
              <div className="folder-preview">
                <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 17 }}>folder</span>
                <span>storage/profiles/{form.slug || 'имя-профиля'}/files</span>
              </div>
              <TextArea
                label="Описание"
                value={form.description}
                onChange={(description) => setForm({ ...form, description })}
              />
              <TextField
                label="Иконка профиля"
                value={form.iconUrl}
                onChange={(iconUrl) => setForm({ ...form, iconUrl })}
              />
              <label className="checkbox-line">
                <input
                  checked={form.isActive}
                  onChange={(event) => setForm({ ...form, isActive: event.target.checked })}
                  type="checkbox"
                />
                <span>Профиль активен для игроков</span>
              </label>
            </motion.section>
          )}

          {editorSection === 'loader' && (
            <motion.section className="wizard-section" key={editorSection} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
              <div className="form-grid two">
                <label>
                  <span>Версия Minecraft</span>
                  <input
                    value={form.gameVersion}
                    onChange={(event) => updateGameVersion(event.target.value)}
                    list="minecraft-version-options"
                  />
                  <datalist id="minecraft-version-options">
                    {loaderCatalog.minecraftVersions.map((version) => (
                      <option key={version} value={version} />
                    ))}
                  </datalist>
                </label>
                <label>
                  <span>Загрузчик</span>
                  <select value={form.loader} onChange={(event) => updateLoader(event.target.value)}>
                    {!loaderCatalog.loaders.some((loader) => loader.id === form.loader) && (
                      <option value={form.loader}>{form.loader} недоступен для этой версии</option>
                    )}
                    {loaderCatalog.loaders.map((loader) => (
                      <option key={loader.id} value={loader.id}>
                        {loader.label}
                      </option>
                    ))}
                  </select>
                </label>
                <label>
                  <span>Версия загрузчика</span>
                  {selectedLoader?.versions.length ? (
                    <select
                      value={form.loaderVersion}
                      onChange={(event) => setForm({ ...form, loaderVersion: event.target.value })}
                      disabled={!selectedLoader.requiresVersion}
                    >
                      {selectedLoader.versions.map((version) => (
                        <option key={version.value || 'none'} value={version.value}>
                          {version.label}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <input
                      value={form.loaderVersion}
                      onChange={(event) => setForm({ ...form, loaderVersion: event.target.value })}
                      disabled={form.loader === 'vanilla'}
                    />
                  )}
                </label>
                <label>
                  <span>Java для версии</span>
                  <input
                    value={form.javaVersion}
                    onChange={(event) => setForm({ ...form, javaVersion: Number(event.target.value) || 17 })}
                    type="number"
                    min={8}
                  />
                </label>
              </div>
              <div className="build-summary">
                <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>inventory_2</span>
                <span>
                  {isLoadingLoaders
                    ? 'Обновляем список версий'
                    : `${loaderLabel(form.loader, loaderCatalog)} ${form.loaderVersion || ''}`.trim()}
                </span>
              </div>
            </motion.section>
          )}

          {editorSection === 'java' && (
            <motion.section className="wizard-section" key={editorSection} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
              <div className="form-grid three">
                <TextField
                  label="Java Windows"
                  value={form.javaPathWindows}
                  onChange={(javaPathWindows) => setForm({ ...form, javaPathWindows })}
                />
                <TextField
                  label="Java Linux"
                  value={form.javaPathLinux}
                  onChange={(javaPathLinux) => setForm({ ...form, javaPathLinux })}
                />
                <TextField
                  label="Java macOS"
                  value={form.javaPathMacos}
                  onChange={(javaPathMacos) => setForm({ ...form, javaPathMacos })}
                />
              </div>
              <TextArea
                label="JVM args"
                value={form.jvmArgs}
                onChange={(jvmArgs) => setForm({ ...form, jvmArgs })}
              />
            </motion.section>
          )}

          {editorSection === 'launch' && (
            <motion.section className="wizard-section" key={editorSection} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
              <TextArea
                label="Команда Windows"
                value={form.launchCommandWindows}
                onChange={(launchCommandWindows) => setForm({ ...form, launchCommandWindows })}
              />
              <TextArea
                label="Команда Linux"
                value={form.launchCommandLinux}
                onChange={(launchCommandLinux) => setForm({ ...form, launchCommandLinux })}
              />
              <TextArea
                label="Команда macOS"
                value={form.launchCommandMacos}
                onChange={(launchCommandMacos) => setForm({ ...form, launchCommandMacos })}
              />
            </motion.section>
          )}

          {editorSection === 'security' && (
            <motion.section className="wizard-section" key={editorSection} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
              <TextArea
                label="Whitelist / не трогать"
                value={preservePathsToText(form.preservePaths)}
                onChange={(value) => setForm({ ...form, preservePaths: textToPreservePaths(value) })}
                rows={8}
              />
              <div className="build-summary">
                <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>shield</span>
                <span>Папки заканчиваются `/`, пути указываются относительно папки установки клиента.</span>
              </div>
              <div className="folder-preview">
                <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 17 }}>lock</span>
                <span>mods, libraries, versions, assets и runtime всегда защищены manifest.</span>
              </div>
            </motion.section>
          )}

          {editorSection === 'build' && (
            <motion.section className="wizard-section" key={editorSection} initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} transition={{ duration: 0.2 }}>
              <div className="guided-build">
                <div className={selectedId ? 'guide-step done' : 'guide-step'}>
                  <strong>1. Создай и сохрани профиль</strong>
                  <span>{selectedId ? 'Профиль сохранён в backend.' : 'Сначала нажми «Создать профиль».'}</span>
                </div>
                <div className={selectedClientReady ? 'guide-step done' : isPreparingSelected ? 'guide-step loading' : 'guide-step'}>
                  <strong>2. Подготовь клиент</strong>
                  <span>
                    {isPreparingSelected
                      ? 'Файлы Minecraft и загрузчика скачиваются на backend.'
                      : selectedClientReady
                        ? 'Клиент скачан и отмечен как готовый.'
                        : 'Нажми «Подготовить клиент и загрузчик».'}
                  </span>
                </div>
                <div className={selectedClientReady ? 'guide-step' : 'guide-step locked'}>
                  <strong>3. Добавь моды</strong>
                  <span>Кидай моды по SFTP в папку `files/mods`, затем собирай manifest.</span>
                </div>
                <div className={selectedClientReady && (selectedProfile?.fileCount ?? 0) > 0 ? 'guide-step done' : 'guide-step locked'}>
                  <strong>4. Собери manifest</strong>
                  <span>Manifest откроет файлы для скачивания лаунчером.</span>
                </div>
              </div>

              <div className="build-card">
                <div>
                  <span>Папка на backend</span>
                  <strong>storage/profiles/{form.slug || 'имя-профиля'}/files</strong>
                </div>
                <div>
                  <span>Файлы в manifest</span>
                  <strong>{selectedProfile?.fileCount ?? 0}</strong>
                </div>
                <div>
                  <span>Размер сборки</span>
                  <strong>{formatBytes(selectedProfile?.totalSize ?? 0)}</strong>
                </div>
                <div>
                  <span>Моды клиента</span>
                  <strong>storage/profiles/{form.slug || 'имя-профиля'}/files/mods</strong>
                </div>
                <div>
                  <span>Whitelist</span>
                  <strong>{normalizePreservePathsForSave(form.preservePaths).length} путей</strong>
                </div>
                <div>
                  <span>Статус клиента</span>
                  <strong>
                    {isPreparingSelected
                      ? 'Файлы скачиваются'
                      : selectedClientReady
                        ? 'Клиент готов'
                        : 'Нужно подготовить клиент'}
                  </strong>
                </div>
                <div>
                  <span>Обновлено</span>
                  <strong>{selectedProfile?.manifestUpdatedAt ? formatDate(selectedProfile.manifestUpdatedAt) : '-'}</strong>
                </div>
              </div>
              <button
                className="secondary-button inline"
                type="button"
                disabled={!selectedProfile || isPreparingSelected}
                onClick={() => selectedProfile && void prepareClient(selectedProfile)}
              >
                <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>download</span>
                {isPreparingSelected ? 'Файлы скачиваются...' : selectedClientReady ? 'Подготовить заново' : 'Подготовить клиент и загрузчик'}
              </button>
              <button
                className="secondary-button inline"
                type="button"
                disabled={!selectedProfile || !selectedClientReady || scanId === selectedProfile.id}
                onClick={() => selectedProfile && void scanProfile(selectedProfile)}
              >
                <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>build</span>
                {selectedClientReady ? 'Собрать manifest на backend' : 'Сначала подготовь клиент'}
              </button>
            </motion.section>
          )}

          </AnimatePresence>
          <div className="form-footer">
            <motion.button whileHover={{ scale: 1.02 }} whileTap={{ scale: 0.95 }} className="primary-button" type="submit" disabled={isSaving}>
              {selectedId ? <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>save</span> : <span className="material-symbols-rounded" aria-hidden="true" style={{ fontSize: 18 }}>check</span>}
              {selectedId ? 'Сохранить профиль' : 'Создать профиль'}
            </motion.button>
          </div>
        </form>
      </section>
    </section>
  );
}

function TextField({
  label,
  value,
  onChange
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <label>
      <span>{label}</span>
      <input value={value} onChange={(event) => onChange(event.target.value)} />
    </label>
  );
}

function TextArea({
  label,
  value,
  onChange,
  rows = 3
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  rows?: number;
}) {
  return (
    <label>
      <span>{label}</span>
      <textarea value={value} onChange={(event) => onChange(event.target.value)} rows={rows} />
    </label>
  );
}

function profileToForm(profile: Profile): ProfileForm {
  return {
    name: profile.name,
    slug: profile.slug,
    description: profile.description ?? '',
    loader: profile.loader,
    gameVersion: profile.gameVersion,
    loaderVersion: profile.loaderVersion ?? '',
    javaVersion: profile.javaVersion,
    jvmArgs: profile.jvmArgs ?? '',
    iconUrl: profile.iconUrl ?? '',
    javaPathWindows: profile.javaPathWindows ?? '',
    javaPathLinux: profile.javaPathLinux ?? '',
    javaPathMacos: profile.javaPathMacos ?? '',
    launchCommandWindows: profile.launchCommandWindows ?? '',
    launchCommandLinux: profile.launchCommandLinux ?? '',
    launchCommandMacos: profile.launchCommandMacos ?? '',
    preservePaths: profile.preservePaths?.length ? profile.preservePaths : [...defaultPreservePaths],
    isActive: profile.isActive
  };
}

function normalizeProfileBeforeSave(profile: ProfileForm): ProfileForm {
  return {
    ...profile,
    slug: profile.slug || folderNameFromTitle(profile.name),
    preservePaths: normalizePreservePathsForSave(profile.preservePaths)
  };
}

function preservePathsToText(paths: string[]) {
  return paths.join('\n');
}

function textToPreservePaths(value: string) {
  return value.split(/\r?\n/);
}

function normalizePreservePathsForSave(paths: string[]) {
  const result: string[] = [];
  const seen = new Set<string>();
  for (const item of paths) {
    const normalized = normalizePreservePath(item);
    if (!normalized || seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    result.push(normalized);
  }
  return result.length ? result : [...defaultPreservePaths];
}

function normalizePreservePath(value: string) {
  const normalized = value.trim().replace(/\\/g, '/').replace(/\/+/g, '/');
  if (!normalized) {
    return '';
  }
  const isDirectory = normalized.endsWith('/');
  const withoutTrailingSlash = normalized.replace(/\/+$/g, '');
  return `${withoutTrailingSlash}${isDirectory ? '/' : ''}`;
}

async function request<T = unknown>(
  path: string,
  options: { method?: string; token?: string; body?: unknown } = {}
): Promise<T> {
  const response = await fetch(`${apiUrl}${path}`, {
    method: options.method ?? 'GET',
    headers: {
      Accept: 'application/json',
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.token ? { Authorization: `Bearer ${options.token}` } : {})
    },
    body: options.body ? JSON.stringify(options.body) : undefined
  });
  if (response.status === 204) {
    return undefined as T;
  }
  const data = (await response.json().catch(() => ({}))) as ApiError & T;
  if (!response.ok) {
    throw new Error(data.message ?? `HTTP ${response.status}`);
  }
  return data;
}

function folderNameFromTitle(value: string) {
  const normalized = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9а-яё_-]+/gi, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
  return transliterate(normalized) || 'profile';
}

function transliterate(value: string) {
  const map: Record<string, string> = {
    а: 'a',
    б: 'b',
    в: 'v',
    г: 'g',
    д: 'd',
    е: 'e',
    ё: 'e',
    ж: 'zh',
    з: 'z',
    и: 'i',
    й: 'y',
    к: 'k',
    л: 'l',
    м: 'm',
    н: 'n',
    о: 'o',
    п: 'p',
    р: 'r',
    с: 's',
    т: 't',
    у: 'u',
    ф: 'f',
    х: 'h',
    ц: 'c',
    ч: 'ch',
    ш: 'sh',
    щ: 'sch',
    ъ: '',
    ы: 'y',
    ь: '',
    э: 'e',
    ю: 'yu',
    я: 'ya'
  };
  return value
    .split('')
    .map((char) => map[char] ?? char)
    .join('')
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
}

function loaderLabel(loaderId: string, catalog: LoaderCatalog) {
  return catalog.loaders.find((loader) => loader.id === loaderId)?.label ?? loaderId;
}

function javaVersionForMinecraft(gameVersion: string) {
  if (gameVersion.startsWith('1.21') || gameVersion.startsWith('1.20.5') || gameVersion.startsWith('1.20.6')) {
    return 21;
  }
  if (gameVersion.startsWith('1.18') || gameVersion.startsWith('1.19') || gameVersion.startsWith('1.20')) {
    return 17;
  }
  if (gameVersion.startsWith('1.17')) {
    return 16;
  }
  return 8;
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : 'Неизвестная ошибка';
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) {
    return '0 B';
  }
  const units = ['B', 'KB', 'MB', 'GB'];
  let amount = value;
  let unitIndex = 0;
  while (amount >= 1024 && unitIndex < units.length - 1) {
    amount /= 1024;
    unitIndex++;
  }
  return `${amount.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function formatDate(value: string) {
  return new Date(value).toLocaleString('ru-RU');
}
