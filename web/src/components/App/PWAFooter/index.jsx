import { ViewList as ViewListIcon } from '@material-ui/icons'
import { useTranslation } from 'react-i18next'
import { StyledMenuButtonWrapper } from 'style/CustomMaterialUiStyles'
import CloseServer from 'components/CloseServer'
import AddDialogButton from 'components/Add'
import AboutDialog from 'components/About'
import SettingsDialogButton from 'components/Settings'

import StyledPWAFooter from './style'

export default function PWAFooter({ onBackToList, isOffline, isLoading }) {
  const { t } = useTranslation()

  return (
    <StyledPWAFooter>
      <StyledMenuButtonWrapper onClick={onBackToList}>
        <ViewListIcon />

        <div>{t('Torrents')}</div>
      </StyledMenuButtonWrapper>

      <AddDialogButton isOffline={isOffline} isLoading={isLoading} />

      <AboutDialog />

      <SettingsDialogButton isOffline={isOffline} isLoading={isLoading} />

      <CloseServer isOffline={isOffline} isLoading={isLoading} />
    </StyledPWAFooter>
  )
}
